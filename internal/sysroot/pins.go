package sysroot

// The sysroot is the static libraries a Linux musl program links, plus their
// headers. Zig supplies the compiler, musl, and the C runtime objects, so
// those are not in this tree.
//
// Mbed TLS stands in for OpenSSL. curl speaks it natively. The three Mbed
// archives together are a few megabytes. OpenSSL's libcrypto.a alone is
// about 30MB, and git only needs HTTPS.
//
// Edges, and why they exist:
//
//	zlib      nothing. curl compresses HTTP bodies with it.
//	mbedtls   nothing. TLS for git. zlib support inside Mbed TLS stays off,
//	          so the two libraries do not depend on each other.
//	ncurses   nothing. htop and zsh read the wide-character library.
//	curl      zlib and mbedtls. HTTP and HTTPS only. brotli, zstd, nghttp2,
//	          libidn2, and libpsl stay out.

func packages() []Package {
	return []Package{
		{Name: "zlib"},
		{Name: "mbedtls"},
		{Name: "ncurses"},
		{Name: "curl", Depends: []string{"zlib", "mbedtls"}},
	}
}

type source struct {
	version string
	url     string
	sha256  string
}

func sources() map[string]source {
	return map[string]source{
		"zlib": {
			version: "1.3.1",
			url:     "https://github.com/madler/zlib/releases/download/v1.3.1/zlib-1.3.1.tar.gz",
			sha256:  "9a93b2b7dfdac77ceba5a558a580e74667dd6fede4585b91eefb60f03b72df23",
		},
		"mbedtls": {
			version: "3.6.4",
			url:     "https://github.com/Mbed-TLS/mbedtls/releases/download/mbedtls-3.6.4/mbedtls-3.6.4.tar.bz2",
			sha256:  "ec35b18a6c593cf98c3e30db8b98ff93e8940a8c4e690e66b41dfc011d678110",
		},
		"ncurses": {
			version: "6.5",
			url:     "https://ftp.gnu.org/gnu/ncurses/ncurses-6.5.tar.gz",
			sha256:  "136d91bc269a9a5785e5f9e980bc76ab57428f604ce3e5a5a90cebc767971cc6",
		},
		"curl": {
			version: "8.15.0",
			url:     "https://curl.se/download/curl-8.15.0.tar.gz",
			sha256:  "d85cfc79dc505ff800cb1d321a320183035011fa08cb301356425d86be8fc53c",
		},
	}
}
