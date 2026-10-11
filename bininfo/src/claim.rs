/*
---
why: This file checks one start claim against header facts.
uses: bininfo
how: bininfo calls this file to judge hermetic, kind, pie, interp, and needed.
---
*/

//! The claim a header makes about how a file starts.
//! A hermetic header names no other file.
//! This file does not read ELF, Mach-O, or PE bytes.

use crate::kind::{self, Kind};
use crate::style::{self, Json};
use std::fmt::Write;

/// Load facts for one file. The check reads this. It does not read the bytes again.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Facts {
    pub file: String,
    pub format: &'static str,
    pub kind: Kind,
    pub interp: Option<String>,
    pub needed: Vec<String>,
    pub pie: bool,
    /// Set when fat slices do not share one load.
    /// None means the hermetic claim comes from kind, interp, and needed.
    pub fat_hermetic: Option<bool>,
    /// Host compare. It does not change the hermetic result.
    pub host: HostMarks,
}

/// One dotted version. `6.8.0-45-generic` keeps the first three numbers.
#[derive(Clone, Copy, Debug, PartialEq, Eq, PartialOrd, Ord)]
pub struct Version {
    pub major: u32,
    pub minor: u32,
    pub patch: u32,
}

/// One CPU and the OS floor recorded for that CPU.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct CpuFloor {
    pub word: &'static str,
    pub arm64e: bool,
    pub minos: Option<Version>,
    pub os_family: &'static str,
}

/// Facts about the machine that runs the check.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct Host {
    pub arch: &'static str,
    pub os: &'static str,
    pub version: Option<Version>,
    pub pac: bool,
}

/// pass, fail, or skip. `verneed` is always skip.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct HostMarks {
    pub cpu: &'static str,
    pub minos: &'static str,
    pub verneed: &'static str,
}

impl HostMarks {
    #[cfg(test)]
    pub fn skip() -> Self {
        Self {
            cpu: "skip",
            minos: "skip",
            verneed: "skip",
        }
    }
}

/// One fact the caller asked the file to match.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Claim {
    Hermetic(bool),
    Kind(Kind),
    Pie(bool),
    Interp(Option<String>),
    Needed(Vec<String>),
}

/// A claim that did not match the file.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Miss {
    pub claim: String,
    pub found: String,
}

pub fn is_hermetic(kind: Kind, interp: Option<&str>, needed: &[String]) -> bool {
    matches!(kind, Kind::Static | Kind::StaticPie) && interp.is_none() && needed.is_empty()
}

pub fn needed_note(format: &str) -> &'static str {
    match format {
        "elf" => "[DT_NEEDED] Library names the loader opens.",
        "macho" => {
            "[LC_LOAD_DYLIB] Library names the loader opens. Also weak, reexport, and upward."
        }
        "pe" => "[IMAGE_IMPORT_DESCRIPTOR] Library names the loader opens.",
        _ => "Library names the loader opens.",
    }
}

pub fn runpath_note(format: &str) -> &'static str {
    match format {
        "elf" => "[DT_RUNPATH] Directories the loader searches.",
        "macho" => "[none] Mach-O has no RUNPATH record.",
        "pe" => "[none] PE has no search-path record.",
        _ => "Directories the loader searches.",
    }
}

pub fn rpath_note(format: &str) -> &'static str {
    match format {
        "elf" => "[DT_RPATH] Old search directories.",
        "macho" => "[LC_RPATH] Directories the loader uses for @rpath.",
        "pe" => "[none] PE has no search-path record.",
        _ => "Old search directories.",
    }
}

pub fn delay_note(format: &str) -> &'static str {
    match format {
        "pe" => "[ImgDelayDescr] DLL names the loader opens on the first call.",
        "elf" => "[none] ELF has no delay-import record.",
        "macho" => "[none] Mach-O has no delay-import record.",
        _ => "DLL names the loader opens on the first call.",
    }
}

pub fn dlopen_fact(format: &str, needed: &[String]) -> Option<bool> {
    match format {
        "elf" => Some(needed.iter().any(|name| is_libdl(name))),
        _ => None,
    }
}

pub fn dlopen_note(format: &str, needed: &[String]) -> &'static str {
    match (format, dlopen_fact(format, needed)) {
        ("elf", Some(true)) => {
            "[DT_NEEDED] A libdl soname is present. This file can call dlopen. The call names the library later."
        }
        ("elf", _) => "[DT_NEEDED] No entry is libdl. The header does not show a dlopen call.",
        ("macho", _) => "[none] libSystem is common. That name does not prove dlopen.",
        ("pe", _) => "[none] kernel32 is common. That name does not prove LoadLibrary.",
        _ => "The header does not show a dlopen call.",
    }
}

fn is_libdl(name: &str) -> bool {
    let base = name.rsplit(['/', '\\']).next().unwrap_or(name);
    base == "libdl.so" || base.starts_with("libdl.so.")
}

pub fn write_delay(out: &mut String, color: bool, indent: usize, format: &str, delay: &[String]) {
    let fg = if delay.is_empty() {
        style::LABEL
    } else {
        style::WHITE
    };
    style::noted_list(
        out,
        color,
        indent,
        "delay",
        delay,
        delay_note(format),
        fg,
        false,
        crate::library::note,
    );
}

pub fn write_dlopen(out: &mut String, color: bool, indent: usize, format: &str, needed: &[String]) {
    let fact = dlopen_fact(format, needed);
    let word = match fact {
        Some(true) => "true",
        Some(false) => "false",
        None => "null",
    };
    let fg = if fact == Some(true) {
        style::WHITE
    } else {
        style::LABEL
    };
    style::field(
        out,
        color,
        indent,
        "dlopen",
        word,
        dlopen_note(format, needed),
        fg,
        false,
    );
}

pub fn write_search(
    out: &mut String,
    color: bool,
    indent: usize,
    format: &str,
    runpath: &[String],
    rpath: &[String],
) {
    search_field(out, color, indent, "runpath", runpath, runpath_note(format));
    search_field(out, color, indent, "rpath", rpath, rpath_note(format));
}

fn search_field(
    out: &mut String,
    color: bool,
    indent: usize,
    key: &str,
    items: &[String],
    note: &str,
) {
    let fg = if items.is_empty() {
        style::LABEL
    } else {
        style::WHITE
    };
    style::path_list(out, color, indent, key, items, note, fg, false);
}

pub fn hermetic_note(kind: Kind, yes: bool) -> &'static str {
    if yes {
        return "The header closure is this file. This check does not read dlopen.";
    }
    match kind {
        Kind::Shared => "A library does not start a run.",
        Kind::Rel => "An object file does not start a run.",
        Kind::Other => "This file is outside the four start kinds.",
        _ => "The header names another file. This check does not read dlopen.",
    }
}

pub fn write_load(
    out: &mut Json,
    kind: Kind,
    interp: Option<&str>,
    needed: &[String],
    runpath: &[String],
    rpath: &[String],
    delay: &[String],
    format: &str,
    pie: bool,
) {
    out.load(
        kind::name(kind),
        interp,
        needed,
        runpath,
        rpath,
        delay,
        dlopen_fact(format, needed),
        pie,
    );
}

pub fn parse(tokens: &[String]) -> Result<Vec<Claim>, String> {
    let mut claims = Vec::new();
    let mut needed: Option<Vec<String>> = None;
    let mut saw_hermetic = false;
    let mut saw_kind = false;
    let mut saw_pie = false;
    let mut saw_interp = false;
    for token in tokens {
        if let Some(name) = token.strip_prefix("needed=") {
            let list = needed.get_or_insert_with(Vec::new);
            if !name.is_empty() {
                list.push(name.to_string());
            }
            continue;
        }
        if token == "hermetic" || token == "hermetic=true" {
            if saw_hermetic {
                return Err("duplicate claim: hermetic".into());
            }
            saw_hermetic = true;
            claims.push(Claim::Hermetic(true));
            continue;
        }
        if let Some(value) = token.strip_prefix("hermetic=") {
            if saw_hermetic {
                return Err("duplicate claim: hermetic".into());
            }
            saw_hermetic = true;
            claims.push(Claim::Hermetic(parse_bool(value)?));
            continue;
        }
        if let Some(value) = token.strip_prefix("kind=") {
            if saw_kind {
                return Err("duplicate claim: kind".into());
            }
            saw_kind = true;
            claims.push(Claim::Kind(parse_kind(value)?));
            continue;
        }
        if let Some(value) = token.strip_prefix("pie=") {
            if saw_pie {
                return Err("duplicate claim: pie".into());
            }
            saw_pie = true;
            claims.push(Claim::Pie(parse_bool(value)?));
            continue;
        }
        if let Some(value) = token.strip_prefix("interp=") {
            if saw_interp {
                return Err("duplicate claim: interp".into());
            }
            saw_interp = true;
            let path = if value == "null" {
                None
            } else {
                Some(value.to_string())
            };
            claims.push(Claim::Interp(path));
            continue;
        }
        return Err(format!("unknown claim: {token}"));
    }
    if let Some(list) = needed {
        claims.push(Claim::Needed(list));
    }
    Ok(claims)
}

pub fn judge(facts: &Facts, claims: &[Claim]) -> Vec<Miss> {
    let mut misses = Vec::new();
    for claim in claims {
        match claim {
            Claim::Hermetic(want) => {
                let got = shown_hermetic(facts);
                if got != *want {
                    misses.push(Miss {
                        claim: format!("hermetic={want}"),
                        found: bool_word(got).into(),
                    });
                }
            }
            Claim::Kind(want) => {
                if facts.kind != *want {
                    misses.push(Miss {
                        claim: format!("kind={}", kind::name(*want)),
                        found: kind::name(facts.kind).into(),
                    });
                }
            }
            Claim::Pie(want) => {
                if facts.pie != *want {
                    misses.push(Miss {
                        claim: format!("pie={want}"),
                        found: bool_word(facts.pie).into(),
                    });
                }
            }
            Claim::Interp(want) => {
                if facts.interp.as_deref() != want.as_deref() {
                    misses.push(Miss {
                        claim: format!("interp={}", want.as_deref().unwrap_or("null")),
                        found: facts.interp.clone().unwrap_or_else(|| "null".into()),
                    });
                }
            }
            Claim::Needed(want) => {
                if &facts.needed != want {
                    misses.push(Miss {
                        claim: format!("needed={}", needed_token(want)),
                        found: style::yaml_list(&facts.needed),
                    });
                }
            }
        }
    }
    misses
}

fn shown_hermetic(facts: &Facts) -> bool {
    facts
        .fat_hermetic
        .unwrap_or_else(|| is_hermetic(facts.kind, facts.interp.as_deref(), &facts.needed))
}

/// First three dotted numbers. A later suffix is ignored.
pub fn parse_version(text: &str) -> Option<Version> {
    let mut parts = [0u32; 3];
    let mut slot = 0usize;
    let mut current: Option<u32> = None;
    let mut saw_digit = false;
    for c in text.trim().chars() {
        if let Some(digit) = c.to_digit(10) {
            let next = current.unwrap_or(0).checked_mul(10)?.checked_add(digit)?;
            current = Some(next);
            saw_digit = true;
        } else if c == '.' && slot < 2 && current.is_some() {
            parts[slot] = current.unwrap();
            current = None;
            slot += 1;
        } else {
            break;
        }
    }
    if !saw_digit {
        return None;
    }
    if let Some(value) = current {
        parts[slot] = value;
    }
    Some(Version {
        major: parts[0],
        minor: parts[1],
        patch: parts[2],
    })
}

/// Compare file floors with this host. The check does not open named files.
pub fn host_marks(floors: &[CpuFloor], host: &Host) -> HostMarks {
    let comparable: Vec<&CpuFloor> = floors
        .iter()
        .filter(|floor| !floor.word.is_empty())
        .collect();
    let cpu = if comparable.is_empty() {
        "skip"
    } else if comparable.iter().any(|floor| cpu_ok(floor, host)) {
        "pass"
    } else {
        "fail"
    };
    let minos = comparable
        .iter()
        .find(|floor| cpu_ok(floor, host) && floor.minos.is_some())
        .map(|floor| minos_mark(floor, host))
        .unwrap_or("skip");
    HostMarks {
        cpu,
        minos,
        verneed: "skip",
    }
}

fn cpu_ok(floor: &CpuFloor, host: &Host) -> bool {
    let arch_ok = match floor.word {
        "x86-64" => host.arch == "x86_64",
        "x86" => host.arch == "x86",
        "AArch64" => host.arch == "aarch64",
        _ => false,
    };
    arch_ok && !(floor.arm64e && !host.pac)
}

fn minos_mark(floor: &CpuFloor, host: &Host) -> &'static str {
    let Some(floor_ver) = floor.minos else {
        return "skip";
    };
    if floor.os_family.is_empty() || floor.os_family != host.os {
        return "skip";
    }
    match host.version {
        Some(host_ver) if floor_ver <= host_ver => "pass",
        Some(_) => "fail",
        None => "skip",
    }
}

fn mark_note(kind: &str, mark: &str) -> &'static str {
    match (kind, mark) {
        ("cpu", "pass") => "This CPU can run this file.",
        ("cpu", "fail") => "This CPU cannot run this file.",
        ("cpu", _) => "The CPU word is absent.",
        ("minos", "pass") => "The file floor is not newer than this OS.",
        ("minos", "fail") => "The file floor is newer than this OS.",
        ("minos", _) => "The floor is for another OS, or the version is unknown.",
        _ => "This check does not open named files.",
    }
}

fn mark_tone(mark: &str) -> (style::Rgb, bool) {
    match mark {
        "pass" => (style::GREEN, true),
        "fail" => (style::PINK, true),
        _ => (style::LABEL, false),
    }
}

pub fn report(facts: &Facts, misses: &[Miss], color: bool) -> String {
    let yes = shown_hermetic(facts);
    let mut out = String::new();
    style::field(
        &mut out,
        color,
        0,
        "file",
        &facts.file,
        "Path of this file.",
        style::WHITE,
        false,
    );
    style::field(
        &mut out,
        color,
        0,
        "format",
        facts.format,
        "elf, macho, or pe.",
        style::FG,
        true,
    );
    let (fg, bold) = style::kind_tone(facts.kind);
    style::field(
        &mut out,
        color,
        0,
        "kind",
        kind::name(facts.kind),
        "Start kind. The same word in each format.",
        fg,
        bold,
    );
    style::field(
        &mut out,
        color,
        0,
        "pie",
        bool_word(facts.pie),
        "Position-independent bit.",
        if facts.pie {
            style::GREEN
        } else {
            style::LABEL
        },
        facts.pie,
    );
    let interp = facts.interp.as_deref().unwrap_or("null");
    style::field(
        &mut out,
        color,
        0,
        "interp",
        interp,
        "Path of the loader. null means this file starts itself.",
        if facts.interp.is_some() {
            style::WHITE
        } else {
            style::LABEL
        },
        false,
    );
    style::noted_list(
        &mut out,
        color,
        0,
        "needed",
        &facts.needed,
        needed_note(facts.format),
        if facts.needed.is_empty() {
            style::LABEL
        } else {
            style::WHITE
        },
        false,
        crate::library::note,
    );
    let (fg, bold) = if yes {
        (style::GREEN, true)
    } else {
        (style::PINK, true)
    };
    style::field(
        &mut out,
        color,
        0,
        "hermetic",
        bool_word(yes),
        hermetic_note(facts.kind, yes),
        fg,
        bold,
    );
    let pass = misses.is_empty();
    style::field(
        &mut out,
        color,
        0,
        "result",
        if pass { "pass" } else { "fail" },
        "pass means every claim matched.",
        if pass { style::GREEN } else { style::PINK },
        true,
    );
    if !misses.is_empty() {
        style::blank(&mut out);
        style::rung(
            &mut out,
            color,
            "miss",
            "A claim that did not match.",
            style::PINK,
            true,
        );
        for miss in misses {
            let _ = writeln!(out, "  - claim: {}", miss.claim);
            let _ = writeln!(out, "    found: {}", miss.found);
        }
    }
    style::blank(&mut out);
    style::rung(
        &mut out,
        color,
        "host",
        "Host facts do not change the hermetic result.",
        style::FG,
        true,
    );
    for (key, mark) in [
        ("cpu", facts.host.cpu),
        ("minos", facts.host.minos),
        ("verneed", facts.host.verneed),
    ] {
        let (fg, bold) = mark_tone(mark);
        style::field(
            &mut out,
            color,
            2,
            key,
            mark,
            mark_note(key, mark),
            fg,
            bold,
        );
    }
    out
}

pub fn report_json(facts: &Facts, misses: &[Miss]) -> String {
    let yes = shown_hermetic(facts);
    let mut out = Json::new();
    out.str("file", &facts.file);
    out.str("format", facts.format);
    out.str("kind", kind::name(facts.kind));
    out.flag("pie", facts.pie);
    out.opt("interp", facts.interp.as_deref());
    out.strings("needed", &facts.needed);
    out.flag("hermetic", yes);
    out.str("result", if misses.is_empty() { "pass" } else { "fail" });
    let rows: Vec<(&str, &str)> = misses
        .iter()
        .map(|miss| (miss.claim.as_str(), miss.found.as_str()))
        .collect();
    out.pairs("miss", "claim", "found", &rows);
    let mut host = Json::new();
    host.str("cpu", facts.host.cpu);
    host.str("minos", facts.host.minos);
    host.str("verneed", facts.host.verneed);
    out.raw("host", &host.finish_inline());
    out.finish()
}

fn parse_kind(token: &str) -> Result<Kind, String> {
    match token {
        "static" => Ok(Kind::Static),
        "dynamic" => Ok(Kind::Dynamic),
        "dynamic-pie" => Ok(Kind::DynamicPie),
        "static-pie" => Ok(Kind::StaticPie),
        "shared" => Ok(Kind::Shared),
        "rel" => Ok(Kind::Rel),
        "other" => Ok(Kind::Other),
        _ => Err(format!("unknown kind: {token}")),
    }
}

fn parse_bool(token: &str) -> Result<bool, String> {
    match token {
        "true" => Ok(true),
        "false" => Ok(false),
        _ => Err(format!("unknown value: {token}")),
    }
}

fn bool_word(value: bool) -> &'static str {
    if value {
        "true"
    } else {
        "false"
    }
}

fn needed_token(names: &[String]) -> String {
    if names.is_empty() {
        "[]".to_string()
    } else {
        names.join(",")
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::style;

    fn facts(kind: Kind, interp: Option<&str>, needed: &[&str], pie: bool) -> Facts {
        Facts {
            file: "bin".into(),
            format: "elf",
            kind,
            interp: interp.map(str::to_string),
            needed: needed.iter().map(|name| (*name).to_string()).collect(),
            pie,
            fat_hermetic: None,
            host: HostMarks::skip(),
        }
    }

    #[test]
    fn a_closed_program_is_hermetic() {
        assert!(is_hermetic(Kind::Static, None, &[]));
        assert!(is_hermetic(Kind::StaticPie, None, &[]));
        assert!(!is_hermetic(Kind::Static, None, &["libtiny.so".into()]));
        assert!(!is_hermetic(Kind::Static, Some("/lib/ld.so"), &[]));
        assert!(!is_hermetic(Kind::Dynamic, Some("/lib/ld.so"), &[]));
        assert!(!is_hermetic(Kind::DynamicPie, None, &["a.dll".into()]));
        assert!(!is_hermetic(Kind::Shared, None, &[]));
        assert!(!is_hermetic(Kind::Rel, None, &[]));
        assert!(!is_hermetic(Kind::Other, None, &[]));
    }

    #[test]
    fn claims_match_or_name_the_miss() {
        let closed = facts(Kind::Static, None, &[], false);
        let claims = parse(&[
            "hermetic".into(),
            "kind=static".into(),
            "pie=false".into(),
            "interp=null".into(),
            "needed=".into(),
        ])
        .unwrap();
        assert!(judge(&closed, &claims).is_empty());

        let open = facts(Kind::DynamicPie, Some("/lib/ld.so"), &["libtiny.so"], true);
        let misses = judge(
            &open,
            &parse(&["hermetic".into(), "needed=other.so".into()]).unwrap(),
        );
        assert_eq!(misses[0].claim, "hermetic=true");
        assert_eq!(misses[0].found, "false");
        assert_eq!(misses[1].claim, "needed=other.so");
        assert_eq!(misses[1].found, "[libtiny.so]");
    }

    #[test]
    fn needed_order_is_exact_and_bad_tokens_fail() {
        let file = facts(
            Kind::DynamicPie,
            Some("/usr/lib/dyld"),
            &["libtiny.dylib", "/usr/lib/libSystem.B.dylib"],
            true,
        );
        let claims = parse(&[
            "needed=libtiny.dylib".into(),
            "needed=/usr/lib/libSystem.B.dylib".into(),
        ])
        .unwrap();
        assert!(judge(&file, &claims).is_empty());
        let swapped = parse(&[
            "needed=/usr/lib/libSystem.B.dylib".into(),
            "needed=libtiny.dylib".into(),
        ])
        .unwrap();
        assert_eq!(judge(&file, &swapped).len(), 1);

        assert!(parse(&["kind=static".into(), "kind=rel".into()]).is_err());
        assert!(parse(&["pie=yes".into()]).is_err());
        assert!(parse(&["nope".into()]).is_err());
    }

    #[test]
    fn the_report_shows_pass_and_fail() {
        let closed = facts(Kind::Static, None, &[], false);
        let pass = report(&closed, &[], false);
        assert!(pass.contains("hermetic: true"), "{pass}");
        assert!(pass.contains("result: pass"), "{pass}");
        assert!(!pass.contains("says:"), "{pass}");
        assert!(!pass.contains("\nmiss:"), "{pass}");

        let misses = judge(&closed, &parse(&["kind=shared".into()]).unwrap());
        let fail = report(&closed, &misses, false);
        assert!(fail.contains("result: fail"), "{fail}");
        assert!(fail.contains("claim: kind=shared"), "{fail}");
        assert!(fail.contains("found: static"), "{fail}");

        let json = report_json(&closed, &misses);
        assert!(!json.contains(r#""says""#), "{json}");
        assert!(json.contains(r#""result":"fail""#), "{json}");
        assert!(json.contains(r#""hermetic":true"#), "{json}");
        assert!(json.contains(r#""claim":"kind=shared""#), "{json}");
        assert!(json.contains(r#""found":"static""#), "{json}");
    }

    #[test]
    fn the_report_prints_needed_as_a_block_list() {
        let open = facts(
            Kind::DynamicPie,
            Some("/lib/ld.so"),
            &["libz.so", "liba.so"],
            true,
        );
        let text = report(&open, &[], false);
        assert!(!text.contains("needed: ["), "{text}");
        let lines: Vec<&str> = text.lines().collect();
        let at = lines
            .iter()
            .position(|line| line.starts_with("needed:"))
            .unwrap();
        assert_eq!(
            lines[at].find('#'),
            Some(style::COLUMN + 2),
            "{}",
            lines[at]
        );
        assert!(
            lines[at].contains("[DT_NEEDED] Library names the loader opens."),
            "{}",
            lines[at]
        );
        assert!(lines[at + 1].contains("  - libz.so"), "{}", lines[at + 1]);
        assert!(
            lines[at + 1].contains("zlib compression."),
            "{}",
            lines[at + 1]
        );
        assert!(lines[at + 2].contains("liba.so"), "{}", lines[at + 2]);
        assert!(
            lines[at + 2].contains("A shared library."),
            "{}",
            lines[at + 2]
        );
        assert!(lines[at + 3].starts_with("hermetic:"), "{}", lines[at + 3]);
    }

    #[test]
    fn fat_slices_that_differ_keep_a_hermetic_claim() {
        let mut facts = facts(Kind::Other, None, &[], false);
        facts.fat_hermetic = Some(true);
        let text = report(&facts, &[], false);
        assert!(text.contains("hermetic: true"), "{text}");
        assert!(text.contains("kind: other"), "{text}");
        assert!(!text.contains("says:"), "{text}");
        let misses = judge(&facts, &parse(&["hermetic".into()]).unwrap());
        assert!(misses.is_empty(), "{misses:?}");
    }

    #[test]
    fn libdl_is_the_soname_not_a_substring() {
        let libdl = |name: &str| vec![name.to_string()];
        assert_eq!(dlopen_fact("elf", &libdl("libdl.so")), Some(true));
        assert_eq!(dlopen_fact("elf", &libdl("/lib/libdl.so.2")), Some(true));
        assert_eq!(dlopen_fact("elf", &libdl("libc.so.6")), Some(false));
        assert_eq!(dlopen_fact("elf", &libdl("libdl2.so")), Some(false));
        assert_eq!(
            dlopen_fact("macho", &libdl("/usr/lib/libSystem.B.dylib")),
            None
        );
        assert_eq!(dlopen_fact("pe", &libdl("kernel32.dll")), None);
    }

    fn ver(major: u32, minor: u32, patch: u32) -> Version {
        Version {
            major,
            minor,
            patch,
        }
    }

    fn floor(
        word: &'static str,
        arm64e: bool,
        minos: Option<Version>,
        os_family: &'static str,
    ) -> CpuFloor {
        CpuFloor {
            word,
            arm64e,
            minos,
            os_family,
        }
    }

    fn host(arch: &'static str, os: &'static str, version: Option<Version>, pac: bool) -> Host {
        Host {
            arch,
            os,
            version,
            pac,
        }
    }

    #[test]
    fn host_marks_compare_the_matching_slice_only() {
        assert_eq!(parse_version("15.6.1"), Some(ver(15, 6, 1)));
        assert_eq!(parse_version("6.8.0-45-generic"), Some(ver(6, 8, 0)));
        assert_eq!(parse_version("nope"), None);

        let mac = host("aarch64", "macos", Some(ver(14, 0, 0)), true);
        let linux = host("aarch64", "linux", Some(ver(6, 8, 0)), false);
        let intel = host("x86_64", "linux", Some(ver(6, 8, 0)), false);

        assert_eq!(
            host_marks(&[floor("x86-64", false, None, "")], &intel).cpu,
            "pass"
        );
        assert_eq!(
            host_marks(&[floor("AArch64", false, None, "")], &mac).cpu,
            "pass"
        );
        assert_eq!(
            host_marks(&[floor("AArch64", true, None, "")], &linux).cpu,
            "fail"
        );
        assert_eq!(
            host_marks(&[floor("AArch64", true, None, "")], &mac).cpu,
            "pass"
        );
        assert_eq!(
            host_marks(&[floor("ARM64_32", false, None, "")], &mac).cpu,
            "fail"
        );
        assert_eq!(host_marks(&[floor("", false, None, "")], &mac).cpu, "skip");

        let newer = host_marks(
            &[floor("x86-64", false, Some(ver(99, 0, 0)), "linux")],
            &intel,
        );
        assert_eq!(newer.minos, "fail");
        assert_eq!(newer.verneed, "skip");

        let fat = host_marks(
            &[
                floor("x86-64", false, Some(ver(99, 0, 0)), "macos"),
                floor("AArch64", false, Some(ver(13, 0, 0)), "macos"),
            ],
            &mac,
        );
        assert_eq!(fat.cpu, "pass");
        assert_eq!(fat.minos, "pass");

        let windows = host_marks(
            &[floor("x86-64", false, Some(ver(10, 0, 0)), "windows")],
            &host("x86_64", "macos", Some(ver(15, 0, 0)), false),
        );
        assert_eq!(windows.cpu, "pass");
        assert_eq!(windows.minos, "skip");

        let ios = host_marks(&[floor("AArch64", false, Some(ver(17, 0, 0)), "ios")], &mac);
        assert_eq!(ios.cpu, "pass");
        assert_eq!(ios.minos, "skip");

        let unknown = host_marks(
            &[floor("AArch64", false, Some(ver(14, 0, 0)), "macos")],
            &host("aarch64", "macos", None, true),
        );
        assert_eq!(unknown.minos, "skip");

        let mut closed = facts(Kind::Static, None, &[], false);
        closed.host = newer;
        let misses = judge(&closed, &parse(&["hermetic".into()]).unwrap());
        assert!(misses.is_empty());
        let text = report(&closed, &misses, false);
        assert!(text.contains("hermetic: true"), "{text}");
        assert!(text.contains("result: pass"), "{text}");
        assert!(text.contains("minos: fail"), "{text}");
        assert!(text.contains("verneed: skip"), "{text}");
        assert!(
            text.contains("Host facts do not change the hermetic result."),
            "{text}"
        );
        assert!(
            text.contains("This check does not open named files."),
            "{text}"
        );
        assert!(!text.contains("says:"), "{text}");
        let lines: Vec<&str> = text.lines().collect();
        let result = lines
            .iter()
            .position(|line| line.starts_with("result:"))
            .unwrap();
        let host_at = lines
            .iter()
            .position(|line| line.starts_with("host:"))
            .unwrap();
        assert!(host_at > result, "{text}");

        let json = report_json(&closed, &[]);
        assert!(
            json.contains(r#""host":{"cpu":"pass","minos":"fail","verneed":"skip"}"#),
            "{json}"
        );
        assert!(json.contains(r#""hermetic":true"#), "{json}");
    }
}
