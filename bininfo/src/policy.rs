/*
---
why: This file reads one policy for each binary.
uses: bininfo
how: bininfo --policy reads a directory. osload calls misses.
---
*/

//! One TOML file is the policy for one binary.
//! The file path under the policy root is the binary path.
//! `needed` lists the library names the loader opens.
//! Each of those names has a why.

use crate::kind;
use std::fs;
use std::path::Path;

/// The policies for one pack. One entry is one binary.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Set {
    bins: Vec<One>,
}

#[derive(Clone, Debug, PartialEq, Eq)]
struct One {
    path: String,
    rule: Rule,
}

#[derive(Clone, Debug, PartialEq, Eq)]
struct Rule {
    format: String,
    universal: Option<String>,
    cpu: Option<Vec<String>>,
    kind: Option<String>,
    pie: Option<bool>,
    hermetic: Option<bool>,
    interp: Option<String>,
    needed: Vec<String>,
    delay: Option<Vec<String>>,
    runpath: Option<Vec<String>>,
    rpath: Option<Vec<String>>,
}

/// One slice of load facts. A thin file has one slice.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct SliceSnap {
    pub kind: &'static str,
    pub pie: bool,
    pub hermetic: bool,
    pub interp: Option<String>,
    pub needed: Vec<String>,
    pub delay: Vec<String>,
    pub runpath: Vec<String>,
    pub rpath: Vec<String>,
}

/// Facts a binary policy reads. `universal` is null for a thin file.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct FileSnap {
    pub format: &'static str,
    pub universal: Option<&'static str>,
    pub cpus: Vec<&'static str>,
    pub slices: Vec<SliceSnap>,
}

/// The policies stored next to this crate.
pub fn devlayer() -> Result<Set, String> {
    parse_set(&[(
        "bin/devlayer",
        include_str!("../policy/bin/devlayer.toml"),
    )])
}

/// Read every `*.toml` file under `root`.
/// The path of the file, without `.toml`, is the binary path.
pub fn load_dir(root: &Path) -> Result<Set, String> {
    let mut found = Vec::new();
    collect(root, root, &mut found)?;
    if found.is_empty() {
        return Err("policy has no binary".into());
    }
    let pairs: Vec<(&str, &str)> = found.iter().map(|(key, text)| (key.as_str(), text.as_str())).collect();
    parse_set(&pairs)
}

/// Build a set from `(binary path, toml)` pairs.
pub fn parse_set(files: &[(&str, &str)]) -> Result<Set, String> {
    let mut bins = Vec::new();
    for (path, text) in files {
        check_key(path)?;
        if bins.iter().any(|one: &One| one.path == *path) {
            return Err(format!("duplicate policy {path}"));
        }
        bins.push(One {
            path: (*path).to_string(),
            rule: parse_one(path, text)?,
        });
    }
    if bins.is_empty() {
        return Err("policy has no binary".into());
    }
    Ok(Set { bins })
}

impl Set {
    /// Exact misses for the binary policy that matches `rel`.
    /// No match is `no policy`.
    pub fn misses(&self, rel: &str, snap: &FileSnap) -> Result<Vec<String>, String> {
        let Some(one) = find(&self.bins, rel) else {
            return Err("no policy".into());
        };
        Ok(rule_misses(&one.rule, snap))
    }
}

fn find<'a>(bins: &'a [One], rel: &str) -> Option<&'a One> {
    bins.iter()
        .filter(|one| rel == one.path || rel.ends_with(&format!("/{}", one.path)))
        .max_by_key(|one| one.path.len())
}

fn collect(root: &Path, dir: &Path, out: &mut Vec<(String, String)>) -> Result<(), String> {
    let entries = fs::read_dir(dir).map_err(|err| format!("cannot read {}: {err}", dir.display()))?;
    for entry in entries {
        let entry = entry.map_err(|err| format!("cannot read {}: {err}", dir.display()))?;
        let path = entry.path();
        let kind = entry
            .file_type()
            .map_err(|err| format!("cannot read {}: {err}", path.display()))?;
        if kind.is_dir() {
            collect(root, &path, out)?;
            continue;
        }
        if !kind.is_file() {
            continue;
        }
        if path.extension().and_then(|ext| ext.to_str()) != Some("toml") {
            continue;
        }
        let rel = path.strip_prefix(root).map_err(|err| err.to_string())?;
        let key = rel.to_string_lossy().replace('\\', "/");
        let Some(key) = key.strip_suffix(".toml") else {
            continue;
        };
        check_key(key)?;
        let text = fs::read_to_string(&path)
            .map_err(|err| format!("cannot read {}: {err}", path.display()))?;
        out.push((key.to_string(), text));
    }
    Ok(())
}

fn check_key(path: &str) -> Result<(), String> {
    if path.is_empty()
        || path.starts_with('/')
        || path.split('/').any(|part| part.is_empty() || part == "..")
    {
        return Err(format!("file path {path} leaves the tree"));
    }
    Ok(())
}

fn parse_one(path: &str, text: &str) -> Result<Rule, String> {
    let root: toml::Value = toml::from_str(text).map_err(|err| err.to_string())?;
    let table = root.as_table().ok_or("root must be a table")?;
    let mut format = None;
    let mut universal = None;
    let mut cpu = None;
    let mut kind_word = None;
    let mut pie = None;
    let mut hermetic = None;
    let mut interp = None;
    let mut needed = None;
    let mut delay = None;
    let mut runpath = None;
    let mut rpath = None;
    for (key, item) in table {
        match key.as_str() {
            "format" => format = Some(one_of("format", item, &["elf", "macho", "pe"])?),
            "universal" => {
                universal = Some(one_of(
                    "universal",
                    item,
                    &["null", "universal", "universal2"],
                )?)
            }
            "cpu" => cpu = Some(strings(path, "cpu", item)?),
            "kind" => {
                let word = req_str("kind", item)?;
                kind::parse_name(&word)?;
                kind_word = Some(word);
            }
            "pie" => pie = Some(req_bool("pie", item)?),
            "hermetic" => hermetic = Some(req_bool("hermetic", item)?),
            "interp" => interp = Some(req_str("interp", item)?),
            "needed" => needed = Some(grants(path, "needed", item)?),
            "delay" => delay = Some(grants(path, "delay", item)?),
            "runpath" => runpath = Some(strings(path, "runpath", item)?),
            "rpath" => rpath = Some(strings(path, "rpath", item)?),
            other => return Err(format!("{path}: unknown field {other}")),
        }
    }
    let Some(format) = format else {
        return Err(format!("{path} needs format"));
    };
    let Some(needed) = needed else {
        return Err(format!("{path} needs needed"));
    };
    Ok(Rule {
        format,
        universal,
        cpu,
        kind: kind_word,
        pie,
        hermetic,
        interp,
        needed,
        delay,
        runpath,
        rpath,
    })
}

fn rule_misses(rule: &Rule, snap: &FileSnap) -> Vec<String> {
    let mut out = Vec::new();
    if snap.format != rule.format {
        push_unique(&mut out, format!("format: found {}", snap.format));
    }
    if let Some(want) = &rule.universal {
        let found = snap.universal.unwrap_or("null");
        if found != want {
            push_unique(&mut out, format!("universal: found {found}"));
        }
    }
    if let Some(want) = &rule.cpu {
        if snap.cpus.iter().map(|cpu| (*cpu).to_string()).collect::<Vec<_>>() != *want {
            push_unique(&mut out, format!("cpu: found {}", list_word_static(&snap.cpus)));
        }
    }
    for slice in &snap.slices {
        if let Some(kind) = &rule.kind {
            if slice.kind != kind {
                push_unique(&mut out, format!("kind: found {}", slice.kind));
            }
        }
        if let Some(pie) = rule.pie {
            if slice.pie != pie {
                push_unique(&mut out, format!("pie: found {}", slice.pie));
            }
        }
        if let Some(hermetic) = rule.hermetic {
            if slice.hermetic != hermetic {
                push_unique(&mut out, format!("hermetic: found {}", slice.hermetic));
            }
        }
        if let Some(want) = &rule.interp {
            let found = slice.interp.as_deref().unwrap_or("null");
            if found != want {
                push_unique(&mut out, format!("interp: found {found}"));
            }
        }
        if slice.needed != rule.needed {
            push_unique(&mut out, format!("needed: found {}", list_word(&slice.needed)));
        }
        push_exact(&mut out, "delay", rule.delay.as_deref(), &slice.delay);
        push_exact(&mut out, "runpath", rule.runpath.as_deref(), &slice.runpath);
        push_exact(&mut out, "rpath", rule.rpath.as_deref(), &slice.rpath);
    }
    out
}

fn push_exact(out: &mut Vec<String>, slot: &str, want: Option<&[String]>, found: &[String]) {
    let Some(want) = want else {
        return;
    };
    if found != want {
        push_unique(out, format!("{slot}: found {}", list_word(found)));
    }
}

fn list_word(names: &[String]) -> String {
    if names.is_empty() {
        return "none".into();
    }
    names.join(", ")
}

fn list_word_static(names: &[&str]) -> String {
    if names.is_empty() {
        return "none".into();
    }
    names.join(", ")
}

fn push_unique(out: &mut Vec<String>, line: String) {
    if !out.iter().any(|item| item == &line) {
        out.push(line);
    }
}

/// The why stays in the policy file. The match uses the name.
fn grants(at: &str, key: &str, value: &toml::Value) -> Result<Vec<String>, String> {
    let array = value
        .as_array()
        .ok_or_else(|| format!("{at}.{key} must be an array"))?;
    let mut out = Vec::new();
    for item in array {
        let table = item
            .as_table()
            .ok_or_else(|| format!("{at}.{key} needs a why"))?;
        let mut name = None;
        let mut why = false;
        for (field, val) in table {
            match field.as_str() {
                "name" => name = Some(grant_text(at, key, "name", val)?),
                "why" => {
                    grant_text(at, key, "why", val)?;
                    why = true;
                }
                other => return Err(format!("{at}.{key}: unknown field {other}")),
            }
        }
        let Some(name) = name else {
            return Err(format!("{at}.{key} needs a name"));
        };
        if !why {
            return Err(format!("{at}.{key} needs a why"));
        }
        out.push(name);
    }
    Ok(out)
}

fn grant_text(at: &str, key: &str, field: &str, value: &toml::Value) -> Result<String, String> {
    let text = value
        .as_str()
        .ok_or_else(|| format!("{at}.{key}.{field} must be a string"))?;
    if text.is_empty() || text.chars().all(char::is_whitespace) {
        return Err(format!("{at}.{key} needs a {field}"));
    }
    Ok(text.to_string())
}

fn strings(at: &str, key: &str, value: &toml::Value) -> Result<Vec<String>, String> {
    let array = value
        .as_array()
        .ok_or_else(|| format!("{at}.{key} must be an array"))?;
    let mut out = Vec::new();
    for item in array {
        let text = item
            .as_str()
            .ok_or_else(|| format!("{at}.{key} must be strings"))?;
        if text.is_empty() {
            return Err(format!("{at}.{key} has an empty name"));
        }
        out.push(text.to_string());
    }
    Ok(out)
}

fn one_of(key: &str, value: &toml::Value, words: &[&str]) -> Result<String, String> {
    let text = req_str(key, value)?;
    if words.contains(&text.as_str()) {
        return Ok(text);
    }
    Err(format!("{key}: found {text}"))
}

fn req_str(key: &str, value: &toml::Value) -> Result<String, String> {
    value
        .as_str()
        .map(str::to_string)
        .ok_or_else(|| format!("{key} must be a string"))
}

fn req_bool(key: &str, value: &toml::Value) -> Result<bool, String> {
    value
        .as_bool()
        .ok_or_else(|| format!("{key} must be a bool"))
}

#[cfg(test)]
mod tests {
    use super::{devlayer, load_dir, parse_set, FileSnap, SliceSnap};
    use std::path::Path;

    fn one(body: &str) -> super::Set {
        parse_set(&[("bin/tool", body)]).unwrap()
    }

    #[test]
    fn one_binary_rejects_an_extra_library() {
        let set = one(
            r#"
format = "elf"
needed = [{ name = "libc.so.6", why = "The C library." }]
"#,
        );
        let good = elf_snap(&["libc.so.6"]);
        assert!(set.misses("root/bin/tool", &good).unwrap().is_empty());
        let bad = elf_snap(&["libiconv.so.2"]);
        let miss = set.misses("bin/tool", &bad).unwrap();
        assert!(
            miss.iter().any(|line| line.contains("libiconv.so.2")),
            "{miss:?}"
        );
    }

    #[test]
    fn a_binary_with_no_policy_is_an_error() {
        let set = one(
            r#"
format = "macho"
needed = []
"#,
        );
        let err = set.misses("bin/other", &mach_snap(&[])).unwrap_err();
        assert!(err.contains("no policy"), "{err}");
    }

    #[test]
    fn the_longer_path_wins() {
        let set = parse_set(&[
            (
                "devlayer",
                "format = \"macho\"\nneeded = []\n",
            ),
            (
                "bin/devlayer",
                "format = \"elf\"\nneeded = []\n",
            ),
        ])
        .unwrap();
        let miss = set.misses("stage/bin/devlayer", &mach_snap(&[])).unwrap();
        assert!(
            miss.iter().any(|line| line.contains("format")),
            "{miss:?}"
        );
    }

    #[test]
    fn a_needed_name_without_a_why_is_an_error() {
        let bare = parse_set(&[("bin/tool", "format = \"elf\"\nneeded = [\"libc.so.6\"]\n")])
            .unwrap_err();
        assert!(bare.contains("needs a why"), "{bare}");
        let missing = parse_set(&[(
            "bin/tool",
            "format = \"elf\"\nneeded = [{ name = \"libc.so.6\" }]\n",
        )])
        .unwrap_err();
        assert!(missing.contains("needs a why"), "{missing}");
    }

    #[test]
    fn universal2_rejects_a_third_cpu_and_a_thin_file() {
        let set = one(
            r#"
format = "macho"
universal = "universal2"
cpu = ["AArch64", "x86-64"]
needed = []
"#,
        );
        let ok = FileSnap {
            format: "macho",
            universal: Some("universal2"),
            cpus: vec!["AArch64", "x86-64"],
            slices: vec![empty_slice()],
        };
        assert!(set.misses("bin/tool", &ok).unwrap().is_empty());
        let third = FileSnap {
            cpus: vec!["AArch64", "x86-64", "PowerPC"],
            ..ok.clone()
        };
        let third_miss = set.misses("bin/tool", &third).unwrap();
        assert!(
            third_miss.iter().any(|line| line.contains("PowerPC")),
            "{third_miss:?}"
        );
        let thin = FileSnap {
            universal: None,
            cpus: vec!["AArch64"],
            ..ok
        };
        let thin_miss = set.misses("bin/tool", &thin).unwrap();
        assert!(
            thin_miss.iter().any(|line| line.contains("null")),
            "{thin_miss:?}"
        );
    }

    #[test]
    fn an_unknown_field_is_an_error() {
        let err = parse_set(&[("bin/tool", "format = \"macho\"\nneeded = []\nhost = \"libz\"\n")])
            .unwrap_err();
        assert!(err.contains("unknown field host"), "{err}");
    }

    #[test]
    fn a_path_that_leaves_the_tree_is_an_error() {
        let err = parse_set(&[("../ls", "format = \"macho\"\nneeded = []\n")]).unwrap_err();
        assert!(err.contains("leaves the tree"), "{err}");
    }

    #[test]
    fn an_empty_rpath_rejects_a_recorded_path() {
        let set = one(
            r#"
format = "elf"
needed = []
rpath = []
"#,
        );
        let mut snap = elf_snap(&[]);
        snap.slices[0].rpath = vec!["/opt/homebrew/lib".into()];
        let miss = set.misses("bin/tool", &snap).unwrap();
        assert_eq!(miss, vec!["rpath: found /opt/homebrew/lib".to_string()]);
    }

    #[test]
    fn a_file_rule_reports_the_found_hermetic_bit() {
        let set = one(
            r#"
format = "macho"
hermetic = true
needed = []
"#,
        );
        let mut snap = mach_snap(&[]);
        snap.slices[0].hermetic = false;
        let miss = set.misses("bin/tool", &snap).unwrap();
        assert!(
            miss.iter().any(|line| line == "hermetic: found false"),
            "{miss:?}"
        );
    }

    #[test]
    fn the_devlayer_policy_is_exact() {
        let set = devlayer().unwrap();
        let snap = cli_snap();
        assert!(set.misses("stage/bin/devlayer", &snap).unwrap().is_empty());
        let err = set.misses("devlayer_/devlayer", &snap).unwrap_err();
        assert!(err.contains("no policy"), "{err}");
        let mut extra = snap;
        extra.slices[0]
            .needed
            .push("/usr/lib/libz.1.dylib".into());
        let miss = set.misses("bin/devlayer", &extra).unwrap();
        assert!(
            miss.iter().any(|line| line.starts_with("needed:")),
            "{miss:?}"
        );
    }

    #[test]
    fn the_policy_directory_loads() {
        let root = Path::new(env!("CARGO_MANIFEST_DIR")).join("policy");
        let set = load_dir(&root).unwrap();
        assert!(set.misses("bin/devlayer", &cli_snap()).unwrap().is_empty());
    }

    #[test]
    fn hostile_policy_text_does_not_panic() {
        let huge = "a".repeat(100_000);
        let samples = ["", "\0", "[", "[format", huge.as_str()];
        for text in samples {
            let _ = parse_set(&[("bin/tool", text)]);
        }
        assert!(parse_set(&[("bin/tool", "")]).is_err());
        assert!(parse_set(&[("bin/tool", "format = \"elf\"\nneeded = [{ name = \"libc.so.6\", why = \" \" }]\n")]).is_err());
    }

    fn elf_snap(needed: &[&str]) -> FileSnap {
        let mut slice = empty_slice();
        slice.needed = needed.iter().map(|name| (*name).to_string()).collect();
        FileSnap {
            format: "elf",
            universal: None,
            cpus: Vec::new(),
            slices: vec![slice],
        }
    }

    fn mach_snap(needed: &[&str]) -> FileSnap {
        let mut slice = empty_slice();
        slice.needed = needed.iter().map(|name| (*name).to_string()).collect();
        FileSnap {
            format: "macho",
            universal: None,
            cpus: vec!["AArch64"],
            slices: vec![slice],
        }
    }

    fn empty_slice() -> SliceSnap {
        SliceSnap {
            kind: "dynamic",
            pie: false,
            hermetic: false,
            interp: None,
            needed: Vec::new(),
            delay: Vec::new(),
            runpath: Vec::new(),
            rpath: Vec::new(),
        }
    }

    fn cli_snap() -> FileSnap {
        FileSnap {
            format: "macho",
            universal: None,
            cpus: vec!["AArch64"],
            slices: vec![SliceSnap {
                kind: "dynamic-pie",
                pie: true,
                hermetic: false,
                interp: Some("/usr/lib/dyld".into()),
                needed: vec![
                    "/usr/lib/libSystem.B.dylib".into(),
                    "/usr/lib/libresolv.9.dylib".into(),
                    "/System/Library/Frameworks/CoreFoundation.framework/Versions/A/CoreFoundation"
                        .into(),
                    "/System/Library/Frameworks/Security.framework/Versions/A/Security".into(),
                ],
                delay: Vec::new(),
                runpath: Vec::new(),
                rpath: Vec::new(),
            }],
        }
    }
}
