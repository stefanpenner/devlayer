#!/bin/sh
# Compile fallbacks.txt into ncurses. The library does not open a terminfo directory.
set -eu

here=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
list_file=$here/fallbacks.txt
alias_file=$here/aliases.ti

die() {
  echo "terminfo: $*" >&2
  exit 1
}

names_comma() {
  awk '
    function bad(msg) {
      print msg > "/dev/stderr"
      failed = 1
      exit 1
    }
    {
      sub(/\r$/, "")
      if ($0 ~ /^[[:space:]]*$/) next
      if ($0 ~ /^[[:space:]]*#/) next
      if (NF != 1) bad("bad terminfo line: " $0)
      if ($1 !~ /^[A-Za-z0-9][A-Za-z0-9.+-]{0,63}$/) bad("bad terminfo name: " $1)
      if (seen[$1]) bad("duplicate terminfo name: " $1)
      seen[$1] = 1
      names = names (names ? "," : "") $1
    }
    END {
      if (failed) exit 1
      if (names == "") bad("terminfo list is empty")
      print names
    }
  ' "$1"
}

do_append() {
  src=$1
  aliases=$2
  dest=$src/misc/terminfo.src
  [ -f "$dest" ] || die "no terminfo.src in $src"
  if grep -q '# devlayer:aliases' "$dest"; then
    return 0
  fi
  printf '\n# devlayer:aliases\n' >> "$dest"
  cat "$aliases" >> "$dest"
}

do_tic() {
  src=$1
  dest=$2
  mkdir -p "$dest"
  if [ -x "$dest/tic" ] && [ -x "$dest/infocmp" ]; then
    printf '%s\n%s\n' "$dest/tic" "$dest/infocmp"
    return 0
  fi
  copy=$dest/src
  if [ -d "$copy" ]; then
    rm -r "$copy"
  fi
  mkdir -p "$copy"
  cp -a "$src/." "$copy/"
  cc=${DEVLAYER_HOST_CC:-clang}
  cxx=${DEVLAYER_HOST_CXX:-$cc}
  # Zig's pinned SDK has no sys/ttydev.h. Apple clang can compile tic.
  if [ "$(uname -s)" = Darwin ]; then
    cc=/usr/bin/clang
    cxx=/usr/bin/clang++
  fi
  (
    cd "$copy"
    unset TERMINFO || true
    unset TERMINFO_DIRS || true
    unset LDFLAGS || true
    unset ARFLAGS || true
    export PATH="/usr/bin:/bin:${PATH:-/usr/bin:/bin}"
    export CC="$cc" CXX="$cxx" BUILD_CC="$cc" CFLAGS="-Os"
    # Apple libtool gets ARFLAGS "unknown". Host tic uses ar.
    export AR=/usr/bin/ar RANLIB=/usr/bin/ranlib cf_cv_ar_flags=cr
    ./configure \
      --without-shared \
      --without-cxx \
      --without-cxx-binding \
      --without-debug \
      --without-ada \
      --without-manpages \
      --without-tests
    make -C include
    make -C ncurses
    make -C progs tic infocmp
  ) >&2
  cp -a "$copy/progs/tic" "$dest/tic"
  cp -a "$copy/progs/infocmp" "$dest/infocmp"
  [ -x "$dest/tic" ] || die "tic did not build"
  [ -x "$dest/infocmp" ] || die "infocmp did not build"
  printf '%s\n%s\n' "$dest/tic" "$dest/infocmp"
}

do_check() {
  dir=$1
  file=$2
  comma=$(names_comma "$file")
  want=$(printf '%s' "$comma" | sed 's/,/ /g')
  mk=$dir/ncurses/Makefile
  [ -f "$mk" ] || die "missing $mk"
  got=$(awk '
    /^FALLBACK_LIST[[:space:]]*=/ {
      sub(/^FALLBACK_LIST[[:space:]]*=[[:space:]]*/, "")
      gsub(/[[:space:]]+/, " ")
      sub(/ $/, "")
      print
      exit
    }
  ' "$mk")
  [ "$got" = "$want" ] || die "FALLBACK_LIST is [$got] want [$want]"
  # Skip *.in and *.hin. Those templates keep the unsubstituted token.
  hit=$(find "$dir/include" "$dir/ncurses" -type f ! -name '*.in' ! -name '*.hin' -exec grep -F "NCURSES_USE_DATABASE" {} + || true)
  printf '%s\n' "$hit" | grep -F "#define NCURSES_USE_DATABASE 0" >/dev/null || die "NCURSES_USE_DATABASE is not 0"
  if printf '%s\n' "$hit" | grep -F "#define NCURSES_USE_DATABASE 1" >/dev/null; then
    die "NCURSES_USE_DATABASE is 1"
  fi
  if printf '%s\n' "$hit" | grep -F "@NCURSES_USE_DATABASE@" >/dev/null; then
    die "NCURSES_USE_DATABASE was not substituted"
  fi
}

case ${1:-} in
  list)
    [ -n "${2:-}" ] || die "usage: configure.sh list <file>"
    names_comma "$2"
    ;;
  append)
    [ -n "${3:-}" ] || die "usage: configure.sh append <src> <aliases>"
    do_append "$2" "$3"
    ;;
  tic)
    [ -n "${3:-}" ] || die "usage: configure.sh tic <src> <dest>"
    do_tic "$2" "$3"
    ;;
  check)
    [ -n "${3:-}" ] || die "usage: configure.sh check <src> <file>"
    do_check "$2" "$3"
    ;;
  *)
    [ -n "${1:-}" ] || die "usage: configure.sh <configure> [args]"
    [ -f "$1" ] || die "configure is missing: $1"
    configure=$1
    shift
    src=$(CDPATH= cd -- "$(dirname "$configure")" && pwd)
    do_append "$src" "$alias_file"
    ticdir=${TMPDIR:-/tmp}/devlayer-host-tic.$$
    tools=$(do_tic "$src" "$ticdir")
    tic=$(printf '%s\n' "$tools" | sed -n '1p')
    infocmp=$(printf '%s\n' "$tools" | sed -n '2p')
    [ -n "$tic" ] && [ -n "$infocmp" ] || die "tic did not report two paths"
    comma=$(names_comma "$list_file")
    set -- "$@" \
      --disable-database \
      --disable-db-install \
      --with-fallbacks="$comma" \
      --with-tic-path="$tic" \
      --with-infocmp-path="$infocmp"
    "$configure" "$@"
    do_check "$src" "$list_file"
    ;;
esac
