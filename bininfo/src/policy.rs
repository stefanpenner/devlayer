/*
---
why: This file reads one policy for each binary.
uses: bininfo
how: bininfo --policy reads a directory. osload calls misses.
---
*/

//! One TOML file is the policy for one binary.
//! The file path under the policy root is the binary path.
//! Each key is the record name in that file.
//! Each library name has a why.
//! A Mach-O grant can name compatibility_version and current_version.

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
enum Rule {
    Elf(ElfRule),
    Macho(MachoRule),
    Pe(PeRule),
}

#[derive(Clone, Debug, PartialEq, Eq)]
struct ElfRule {
    pt_interp: String,
    dt_needed: Vec<String>,
    dt_runpath: Vec<String>,
    dt_rpath: Vec<String>,
    df_1_pie: bool,
}

#[derive(Clone, Debug, PartialEq, Eq)]
struct MachoRule {
    cputype: Vec<String>,
    cpusubtype: Option<Vec<String>>,
    filetype: String,
    flags: Option<Vec<String>>,
    mh_pie: bool,
    lc_load_dylinker: String,
    lc_load_dylib: Vec<DylibGrant>,
    lc_load_weak_dylib: Vec<DylibGrant>,
    lc_reexport_dylib: Vec<DylibGrant>,
    lc_load_upward_dylib: Vec<DylibGrant>,
    lc_rpath: Vec<String>,
}

/// One Mach-O dylib grant. Versions are absent when the policy does not name them.
#[derive(Clone, Debug, PartialEq, Eq)]
struct DylibGrant {
    name: String,
    compatibility_version: Option<String>,
    current_version: Option<String>,
}

#[derive(Clone, Debug, PartialEq, Eq)]
struct PeRule {
    image_import_descriptor: Vec<String>,
    img_delay_descr: Vec<String>,
    dynamic_base: bool,
}

/// Records one policy compares. The key is the record name.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Records {
    Elf(ElfSnap),
    Macho(MachoSnap),
    Pe(PeSnap),
}

/// ELF program headers and dynamic tags.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct ElfSnap {
    pub pt_interp: Option<String>,
    pub dt_needed: Vec<String>,
    pub dt_runpath: Vec<String>,
    pub dt_rpath: Vec<String>,
    pub df_1_pie: bool,
}

/// Mach-O header and load commands. One slice is one thin image.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct MachoSnap {
    pub cputype: Vec<&'static str>,
    pub cpusubtype: Vec<&'static str>,
    pub filetype: Vec<String>,
    pub slices: Vec<MachoSlice>,
}

/// One `LC_LOAD_DYLIB` row. Weak, reexport, and upward use the same row.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct MachoDylib {
    pub name: String,
    pub compatibility_version: String,
    pub current_version: String,
}

/// Load commands of one Mach-O slice.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct MachoSlice {
    pub mh_pie: bool,
    pub flags: Vec<String>,
    pub lc_load_dylinker: Option<String>,
    pub lc_load_dylib: Vec<MachoDylib>,
    pub lc_load_weak_dylib: Vec<MachoDylib>,
    pub lc_reexport_dylib: Vec<MachoDylib>,
    pub lc_load_upward_dylib: Vec<MachoDylib>,
    pub lc_rpath: Vec<String>,
}

/// PE import records.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct PeSnap {
    pub image_import_descriptor: Vec<String>,
    pub img_delay_descr: Vec<String>,
    pub dynamic_base: bool,
}

/// Facts a binary policy reads.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct FileSnap {
    pub records: Records,
}

/// The policies stored next to this crate.
pub fn devlayer() -> Result<Set, String> {
    parse_set(&[("bin/devlayer", include_str!("../policy/bin/devlayer.toml"))])
}

/// Read every `*.toml` file under `root`.
/// The path of the file, without `.toml`, is the binary path.
pub fn load_dir(root: &Path) -> Result<Set, String> {
    let mut found = Vec::new();
    collect(root, root, &mut found)?;
    if found.is_empty() {
        return Err("policy has no binary".into());
    }
    let pairs: Vec<(&str, &str)> = found
        .iter()
        .map(|(key, text)| (key.as_str(), text.as_str()))
        .collect();
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
    let entries =
        fs::read_dir(dir).map_err(|err| format!("cannot read {}: {err}", dir.display()))?;
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
        // A symlink to a file is the policy text.
        // A symlink to a directory stays unread.
        let file = if kind.is_file() {
            true
        } else if kind.is_symlink() {
            matches!(fs::metadata(&path), Ok(meta) if meta.is_file())
        } else {
            false
        };
        if !file {
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
    let format = match table.get("format") {
        Some(value) => one_of("format", value, &["elf", "macho", "pe"])?,
        None => return Err(format!("{path} needs format")),
    };
    match format.as_str() {
        "elf" => parse_elf(path, table),
        "macho" => parse_macho(path, table),
        "pe" => parse_pe(path, table),
        _ => Err(format!("format: found {format}")),
    }
}

fn parse_elf(path: &str, table: &toml::map::Map<String, toml::Value>) -> Result<Rule, String> {
    let mut pt_interp = None;
    let mut dt_needed = None;
    let mut dt_runpath = None;
    let mut dt_rpath = None;
    let mut df_1_pie = None;
    for (key, item) in table {
        match key.as_str() {
            "format" => {}
            "PT_INTERP" => pt_interp = Some(path_text("PT_INTERP", item)?),
            "DT_NEEDED" => dt_needed = Some(grants(path, "DT_NEEDED", item)?),
            "DT_RUNPATH" => dt_runpath = Some(strings(path, "DT_RUNPATH", item)?),
            "DT_RPATH" => dt_rpath = Some(strings(path, "DT_RPATH", item)?),
            "DF_1_PIE" => df_1_pie = Some(req_bool("DF_1_PIE", item)?),
            other => return Err(format!("{path}: unknown field {other}")),
        }
    }
    let Some(dt_needed) = dt_needed else {
        return Err(format!("{path} needs DT_NEEDED"));
    };
    Ok(Rule::Elf(ElfRule {
        pt_interp: pt_interp.unwrap_or_else(|| "null".into()),
        dt_needed,
        dt_runpath: dt_runpath.unwrap_or_default(),
        dt_rpath: dt_rpath.unwrap_or_default(),
        df_1_pie: df_1_pie.unwrap_or(false),
    }))
}

fn parse_macho(path: &str, table: &toml::map::Map<String, toml::Value>) -> Result<Rule, String> {
    let mut cputype = None;
    let mut cpusubtype = None;
    let mut filetype = None;
    let mut flags = None;
    let mut mh_pie = None;
    let mut lc_load_dylinker = None;
    let mut lc_load_dylib = None;
    let mut lc_load_weak_dylib = None;
    let mut lc_reexport_dylib = None;
    let mut lc_load_upward_dylib = None;
    let mut lc_rpath = None;
    for (key, item) in table {
        match key.as_str() {
            "format" => {}
            "cputype" => cputype = Some(strings(path, "cputype", item)?),
            "cpusubtype" => cpusubtype = Some(strings(path, "cpusubtype", item)?),
            "filetype" => {
                filetype = Some(one_of(
                    "filetype",
                    item,
                    &[
                        "MH_OBJECT",
                        "MH_EXECUTE",
                        "MH_FVMLIB",
                        "MH_CORE",
                        "MH_PRELOAD",
                        "MH_DYLIB",
                        "MH_DYLINKER",
                        "MH_BUNDLE",
                    ],
                )?)
            }
            "flags" => flags = Some(flag_list(path, item)?),
            "MH_PIE" => mh_pie = Some(req_bool("MH_PIE", item)?),
            "LC_LOAD_DYLINKER" => lc_load_dylinker = Some(path_text("LC_LOAD_DYLINKER", item)?),
            "LC_LOAD_DYLIB" => lc_load_dylib = Some(dylib_grants(path, "LC_LOAD_DYLIB", item)?),
            "LC_LOAD_WEAK_DYLIB" => {
                lc_load_weak_dylib = Some(dylib_grants(path, "LC_LOAD_WEAK_DYLIB", item)?)
            }
            "LC_REEXPORT_DYLIB" => {
                lc_reexport_dylib = Some(dylib_grants(path, "LC_REEXPORT_DYLIB", item)?)
            }
            "LC_LOAD_UPWARD_DYLIB" => {
                lc_load_upward_dylib = Some(dylib_grants(path, "LC_LOAD_UPWARD_DYLIB", item)?)
            }
            "LC_RPATH" => lc_rpath = Some(strings(path, "LC_RPATH", item)?),
            other => return Err(format!("{path}: unknown field {other}")),
        }
    }
    let Some(cputype) = cputype else {
        return Err(format!("{path} needs cputype"));
    };
    let Some(filetype) = filetype else {
        return Err(format!("{path} needs filetype"));
    };
    let Some(lc_load_dylib) = lc_load_dylib else {
        return Err(format!("{path} needs LC_LOAD_DYLIB"));
    };
    let mh_pie = mh_pie.unwrap_or(false);
    if let Some(flags) = &flags {
        let listed = flags.iter().any(|bit| bit == "MH_PIE");
        if listed != mh_pie {
            return Err(format!("{path}: MH_PIE does not match flags"));
        }
    }
    Ok(Rule::Macho(MachoRule {
        cputype,
        cpusubtype,
        filetype,
        flags,
        mh_pie,
        lc_load_dylinker: lc_load_dylinker.unwrap_or_else(|| "null".into()),
        lc_load_dylib,
        lc_load_weak_dylib: lc_load_weak_dylib.unwrap_or_default(),
        lc_reexport_dylib: lc_reexport_dylib.unwrap_or_default(),
        lc_load_upward_dylib: lc_load_upward_dylib.unwrap_or_default(),
        lc_rpath: lc_rpath.unwrap_or_default(),
    }))
}

fn parse_pe(path: &str, table: &toml::map::Map<String, toml::Value>) -> Result<Rule, String> {
    let mut image_import_descriptor = None;
    let mut img_delay_descr = None;
    let mut dynamic_base = None;
    for (key, item) in table {
        match key.as_str() {
            "format" => {}
            "IMAGE_IMPORT_DESCRIPTOR" => {
                image_import_descriptor = Some(grants(path, "IMAGE_IMPORT_DESCRIPTOR", item)?)
            }
            "ImgDelayDescr" => img_delay_descr = Some(grants(path, "ImgDelayDescr", item)?),
            "IMAGE_DLLCHARACTERISTICS_DYNAMIC_BASE" => {
                dynamic_base = Some(req_bool("IMAGE_DLLCHARACTERISTICS_DYNAMIC_BASE", item)?)
            }
            other => return Err(format!("{path}: unknown field {other}")),
        }
    }
    let Some(image_import_descriptor) = image_import_descriptor else {
        return Err(format!("{path} needs IMAGE_IMPORT_DESCRIPTOR"));
    };
    Ok(Rule::Pe(PeRule {
        image_import_descriptor,
        img_delay_descr: img_delay_descr.unwrap_or_default(),
        dynamic_base: dynamic_base.unwrap_or(false),
    }))
}

fn rule_misses(rule: &Rule, snap: &FileSnap) -> Vec<String> {
    match (rule, &snap.records) {
        (Rule::Elf(rule), Records::Elf(snap)) => elf_misses(rule, snap),
        (Rule::Macho(rule), Records::Macho(snap)) => macho_misses(rule, snap),
        (Rule::Pe(rule), Records::Pe(snap)) => pe_misses(rule, snap),
        (_, records) => vec![format!("format: found {}", format_word(records))],
    }
}

fn format_word(records: &Records) -> &'static str {
    match records {
        Records::Elf(_) => "elf",
        Records::Macho(_) => "macho",
        Records::Pe(_) => "pe",
    }
}

fn elf_misses(rule: &ElfRule, snap: &ElfSnap) -> Vec<String> {
    let mut out = Vec::new();
    let found = snap.pt_interp.as_deref().unwrap_or("null");
    if found != rule.pt_interp {
        push_unique(&mut out, format!("PT_INTERP: found {found}"));
    }
    differ(&mut out, "DT_NEEDED", &rule.dt_needed, &snap.dt_needed);
    differ(&mut out, "DT_RUNPATH", &rule.dt_runpath, &snap.dt_runpath);
    differ(&mut out, "DT_RPATH", &rule.dt_rpath, &snap.dt_rpath);
    if snap.df_1_pie != rule.df_1_pie {
        push_unique(&mut out, format!("DF_1_PIE: found {}", snap.df_1_pie));
    }
    out
}

fn macho_misses(rule: &MachoRule, snap: &MachoSnap) -> Vec<String> {
    let mut out = Vec::new();
    let found: Vec<String> = snap.cputype.iter().map(|cpu| (*cpu).to_string()).collect();
    if found != rule.cputype {
        push_unique(&mut out, format!("cputype: found {}", list_word(&found)));
    }
    if let Some(want) = &rule.cpusubtype {
        let found: Vec<String> = snap
            .cpusubtype
            .iter()
            .map(|cpu| (*cpu).to_string())
            .collect();
        if found != *want {
            push_unique(&mut out, format!("cpusubtype: found {}", list_word(&found)));
        }
    }
    if snap.filetype.iter().any(|ty| ty != &rule.filetype) || snap.filetype.is_empty() {
        push_unique(
            &mut out,
            format!("filetype: found {}", list_word(&snap.filetype)),
        );
    }
    for slice in &snap.slices {
        if let Some(want) = &rule.flags {
            if slice.flags != *want {
                push_unique(
                    &mut out,
                    format!("flags: found {}", list_word(&slice.flags)),
                );
            }
        }
        let linker = slice.lc_load_dylinker.as_deref().unwrap_or("null");
        if linker != rule.lc_load_dylinker {
            push_unique(&mut out, format!("LC_LOAD_DYLINKER: found {linker}"));
        }
        dylib_misses(
            &mut out,
            "LC_LOAD_DYLIB",
            &rule.lc_load_dylib,
            &slice.lc_load_dylib,
        );
        dylib_misses(
            &mut out,
            "LC_LOAD_WEAK_DYLIB",
            &rule.lc_load_weak_dylib,
            &slice.lc_load_weak_dylib,
        );
        dylib_misses(
            &mut out,
            "LC_REEXPORT_DYLIB",
            &rule.lc_reexport_dylib,
            &slice.lc_reexport_dylib,
        );
        dylib_misses(
            &mut out,
            "LC_LOAD_UPWARD_DYLIB",
            &rule.lc_load_upward_dylib,
            &slice.lc_load_upward_dylib,
        );
        differ(&mut out, "LC_RPATH", &rule.lc_rpath, &slice.lc_rpath);
        if slice.mh_pie != rule.mh_pie {
            push_unique(&mut out, format!("MH_PIE: found {}", slice.mh_pie));
        }
    }
    out
}

fn dylib_misses(out: &mut Vec<String>, slot: &str, want: &[DylibGrant], found: &[MachoDylib]) {
    let names_match = want.len() == found.len()
        && want
            .iter()
            .zip(found)
            .all(|(row, got)| row.name == got.name);
    if !names_match {
        let names: Vec<String> = found.iter().map(|row| row.name.clone()).collect();
        push_unique(out, format!("{slot}: found {}", list_word(&names)));
        return;
    }
    for (row, got) in want.iter().zip(found) {
        if let Some(ver) = &row.compatibility_version {
            if ver != &got.compatibility_version {
                push_unique(
                    out,
                    format!(
                        "{slot}: found {} compatibility_version {}",
                        got.name, got.compatibility_version
                    ),
                );
            }
        }
        if let Some(ver) = &row.current_version {
            if ver != &got.current_version {
                push_unique(
                    out,
                    format!(
                        "{slot}: found {} current_version {}",
                        got.name, got.current_version
                    ),
                );
            }
        }
    }
}

fn pe_misses(rule: &PeRule, snap: &PeSnap) -> Vec<String> {
    let mut out = Vec::new();
    differ(
        &mut out,
        "IMAGE_IMPORT_DESCRIPTOR",
        &rule.image_import_descriptor,
        &snap.image_import_descriptor,
    );
    differ(
        &mut out,
        "ImgDelayDescr",
        &rule.img_delay_descr,
        &snap.img_delay_descr,
    );
    if snap.dynamic_base != rule.dynamic_base {
        push_unique(
            &mut out,
            format!(
                "IMAGE_DLLCHARACTERISTICS_DYNAMIC_BASE: found {}",
                snap.dynamic_base
            ),
        );
    }
    out
}

fn differ(out: &mut Vec<String>, slot: &str, want: &[String], found: &[String]) {
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

fn dylib_grants(at: &str, key: &str, value: &toml::Value) -> Result<Vec<DylibGrant>, String> {
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
        let mut compatibility_version = None;
        let mut current_version = None;
        for (field, val) in table {
            match field.as_str() {
                "name" => name = Some(grant_text(at, key, "name", val)?),
                "why" => {
                    grant_text(at, key, "why", val)?;
                    why = true;
                }
                "compatibility_version" => {
                    compatibility_version = Some(grant_text(at, key, "compatibility_version", val)?)
                }
                "current_version" => {
                    current_version = Some(grant_text(at, key, "current_version", val)?)
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
        match (&compatibility_version, &current_version) {
            (None, None) | (Some(_), Some(_)) => {}
            (None, Some(_)) => return Err(format!("{at}.{key} needs compatibility_version")),
            (Some(_), None) => return Err(format!("{at}.{key} needs current_version")),
        }
        out.push(DylibGrant {
            name,
            compatibility_version,
            current_version,
        });
    }
    Ok(out)
}

fn flag_list(path: &str, value: &toml::Value) -> Result<Vec<String>, String> {
    let names = strings(path, "flags", value)?;
    let mut seen = Vec::new();
    for name in &names {
        if seen.iter().any(|old| old == name) {
            return Err(format!("{path}: flags has a duplicate"));
        }
        seen.push(name.clone());
    }
    Ok(names)
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

fn path_text(key: &str, value: &toml::Value) -> Result<String, String> {
    let text = req_str(key, value)?;
    if text.is_empty() {
        return Err(format!("{key} has an empty name"));
    }
    Ok(text)
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
    use super::{
        devlayer, load_dir, parse_set, ElfSnap, FileSnap, MachoDylib, MachoSlice, MachoSnap,
        PeSnap, Records,
    };
    use std::path::Path;

    fn one(body: &str) -> super::Set {
        parse_set(&[("bin/tool", body)]).unwrap()
    }

    #[test]
    fn one_binary_rejects_an_extra_library() {
        let set = one(r#"
format = "elf"
DT_NEEDED = [{ name = "libc.so.6", why = "The C library." }]
"#);
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
    fn the_alias_needed_is_an_error() {
        let err = parse_set(&[(
            "bin/tool",
            "format = \"elf\"\nneeded = []\nDT_NEEDED = []\n",
        )])
        .unwrap_err();
        assert!(err.contains("unknown field needed"), "{err}");
    }

    #[test]
    fn a_binary_with_no_policy_is_an_error() {
        let set = one(r#"
format = "macho"
cputype = ["CPU_TYPE_ARM64"]
filetype = "MH_EXECUTE"
LC_LOAD_DYLIB = []
"#);
        let err = set
            .misses("bin/other", &mach_snap(&["CPU_TYPE_ARM64"]))
            .unwrap_err();
        assert!(err.contains("no policy"), "{err}");
    }

    #[test]
    fn the_longer_path_wins() {
        let set = parse_set(&[
            (
                "devlayer",
                "format = \"macho\"\ncputype = [\"CPU_TYPE_ARM64\"]\nfiletype = \"MH_EXECUTE\"\nLC_LOAD_DYLIB = []\n",
            ),
            ("bin/devlayer", "format = \"elf\"\nDT_NEEDED = []\n"),
        ])
        .unwrap();
        let miss = set
            .misses("stage/bin/devlayer", &mach_snap(&["CPU_TYPE_ARM64"]))
            .unwrap();
        assert!(miss.iter().any(|line| line.contains("format")), "{miss:?}");
    }

    #[test]
    fn a_record_name_without_a_why_is_an_error() {
        let bare = parse_set(&[(
            "bin/tool",
            "format = \"elf\"\nDT_NEEDED = [\"libc.so.6\"]\n",
        )])
        .unwrap_err();
        assert!(bare.contains("needs a why"), "{bare}");
        let missing = parse_set(&[(
            "bin/tool",
            "format = \"elf\"\nDT_NEEDED = [{ name = \"libc.so.6\" }]\n",
        )])
        .unwrap_err();
        assert!(missing.contains("needs a why"), "{missing}");
    }

    #[test]
    fn cputype_list_is_exact() {
        let set = one(r#"
format = "macho"
cputype = ["CPU_TYPE_ARM64", "CPU_TYPE_X86_64"]
filetype = "MH_EXECUTE"
LC_LOAD_DYLIB = []
"#);
        let ok = mach_snap(&["CPU_TYPE_ARM64", "CPU_TYPE_X86_64"]);
        assert!(set.misses("bin/tool", &ok).unwrap().is_empty());
        let third = mach_snap(&["CPU_TYPE_ARM64", "CPU_TYPE_X86_64", "CPU_TYPE_POWERPC"]);
        let third_miss = set.misses("bin/tool", &third).unwrap();
        assert!(
            third_miss
                .iter()
                .any(|line| line.contains("CPU_TYPE_POWERPC")),
            "{third_miss:?}"
        );
        let thin = mach_snap(&["CPU_TYPE_ARM64"]);
        let thin_miss = set.misses("bin/tool", &thin).unwrap();
        assert!(
            thin_miss.iter().any(|line| line.starts_with("cputype:")),
            "{thin_miss:?}"
        );
    }

    #[test]
    fn an_unknown_field_is_an_error() {
        let err = parse_set(&[(
            "bin/tool",
            "format = \"macho\"\ncputype = [\"CPU_TYPE_ARM64\"]\nfiletype = \"MH_EXECUTE\"\nLC_LOAD_DYLIB = []\nhost = \"libz\"\n",
        )])
        .unwrap_err();
        assert!(err.contains("unknown field host"), "{err}");
    }

    #[test]
    fn a_path_that_leaves_the_tree_is_an_error() {
        let err = parse_set(&[(
            "../ls",
            "format = \"macho\"\ncputype = [\"CPU_TYPE_ARM64\"]\nfiletype = \"MH_EXECUTE\"\nLC_LOAD_DYLIB = []\n",
        )])
        .unwrap_err();
        assert!(err.contains("leaves the tree"), "{err}");
    }

    #[test]
    fn an_empty_rpath_rejects_a_recorded_path() {
        let set = one(r#"
format = "elf"
DT_NEEDED = []
DT_RPATH = []
"#);
        let mut snap = elf_snap(&[]);
        let Records::Elf(elf) = &mut snap.records else {
            panic!("elf");
        };
        elf.dt_rpath = vec!["/opt/homebrew/lib".into()];
        let miss = set.misses("bin/tool", &snap).unwrap();
        assert_eq!(miss, vec!["DT_RPATH: found /opt/homebrew/lib".to_string()]);
    }

    #[test]
    fn a_clear_pie_bit_reports_the_found_bit() {
        let set = one(r#"
format = "macho"
cputype = ["CPU_TYPE_ARM64"]
filetype = "MH_EXECUTE"
MH_PIE = false
LC_LOAD_DYLIB = []
"#);
        let mut snap = mach_snap(&["CPU_TYPE_ARM64"]);
        let Records::Macho(mach) = &mut snap.records else {
            panic!("macho");
        };
        mach.slices[0].mh_pie = true;
        let miss = set.misses("bin/tool", &snap).unwrap();
        assert!(
            miss.iter().any(|line| line == "MH_PIE: found true"),
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
        let Records::Macho(mach) = &mut extra.records else {
            panic!("macho");
        };
        mach.slices[0]
            .lc_load_dylib
            .push(dylib("/usr/lib/libz.1.dylib", "0.0.0", "0.0.0"));
        let miss = set.misses("bin/devlayer", &extra).unwrap();
        assert!(
            miss.iter().any(|line| line.starts_with("LC_LOAD_DYLIB:")),
            "{miss:?}"
        );
    }

    #[test]
    fn the_policy_directory_loads() {
        let root = Path::new(env!("CARGO_MANIFEST_DIR")).join("policy");
        let set = load_dir(&root).unwrap();
        assert!(set.misses("bin/devlayer", &cli_snap()).unwrap().is_empty());
    }

    #[cfg(unix)]
    #[test]
    fn a_file_symlink_is_a_policy() {
        let root = std::env::temp_dir().join(format!("policy-link-{}", std::process::id()));
        let real = std::env::temp_dir().join(format!("policy-real-{}", std::process::id()));
        let outside = std::env::temp_dir().join(format!("policy-out-{}", std::process::id()));
        let _ = std::fs::remove_dir_all(&root);
        let _ = std::fs::remove_file(&real);
        let _ = std::fs::remove_dir_all(&outside);
        std::fs::create_dir_all(root.join("bin")).unwrap();
        std::fs::write(&real, include_str!("../policy/bin/devlayer.toml")).unwrap();
        std::os::unix::fs::symlink(&real, root.join("bin/devlayer.toml")).unwrap();
        std::fs::create_dir_all(&outside).unwrap();
        std::fs::write(
            outside.join("evil.toml"),
            "format = \"elf\"\nDT_NEEDED = []\n",
        )
        .unwrap();
        std::os::unix::fs::symlink(&outside, root.join("out")).unwrap();
        let set = load_dir(&root).unwrap();
        assert!(set.misses("bin/devlayer", &cli_snap()).unwrap().is_empty());
        let err = set.misses("out/evil", &cli_snap()).unwrap_err();
        assert!(err.contains("no policy"), "{err}");
        let _ = std::fs::remove_dir_all(&root);
        let _ = std::fs::remove_file(&real);
        let _ = std::fs::remove_dir_all(&outside);
    }

    #[test]
    fn hostile_policy_text_does_not_panic() {
        let huge = "a".repeat(100_000);
        let samples = ["", "\0", "[", "[format", huge.as_str()];
        for text in samples {
            let _ = parse_set(&[("bin/tool", text)]);
        }
        assert!(parse_set(&[("bin/tool", "")]).is_err());
        assert!(parse_set(&[(
            "bin/tool",
            "format = \"elf\"\nDT_NEEDED = [{ name = \"libc.so.6\", why = \" \" }]\n"
        )])
        .is_err());
    }

    fn elf_snap(needed: &[&str]) -> FileSnap {
        FileSnap {
            records: Records::Elf(ElfSnap {
                pt_interp: None,
                dt_needed: needed.iter().map(|name| (*name).to_string()).collect(),
                dt_runpath: Vec::new(),
                dt_rpath: Vec::new(),
                df_1_pie: false,
            }),
        }
    }

    fn mach_snap(cpus: &[&'static str]) -> FileSnap {
        FileSnap {
            records: Records::Macho(MachoSnap {
                cputype: cpus.to_vec(),
                cpusubtype: vec!["CPU_SUBTYPE_ARM64_ALL"; cpus.len()],
                filetype: vec!["MH_EXECUTE".into(); cpus.len()],
                slices: vec![empty_mach(); cpus.len()],
            }),
        }
    }

    fn empty_mach() -> MachoSlice {
        MachoSlice {
            mh_pie: false,
            flags: Vec::new(),
            lc_load_dylinker: None,
            lc_load_dylib: Vec::new(),
            lc_load_weak_dylib: Vec::new(),
            lc_reexport_dylib: Vec::new(),
            lc_load_upward_dylib: Vec::new(),
            lc_rpath: Vec::new(),
        }
    }

    fn dylib(name: &str, compat: &str, current: &str) -> MachoDylib {
        MachoDylib {
            name: name.into(),
            compatibility_version: compat.into(),
            current_version: current.into(),
        }
    }

    fn cli_snap() -> FileSnap {
        FileSnap {
            records: Records::Macho(MachoSnap {
                cputype: vec!["CPU_TYPE_ARM64"],
                cpusubtype: vec!["CPU_SUBTYPE_ARM64_ALL"],
                filetype: vec!["MH_EXECUTE".into()],
                slices: vec![MachoSlice {
                    mh_pie: true,
                    flags: vec!["MH_DYLDLINK".into(), "MH_PIE".into()],
                    lc_load_dylinker: Some("/usr/lib/dyld".into()),
                    lc_load_dylib: vec![
                        dylib("/usr/lib/libSystem.B.dylib", "0.0.0", "0.0.0"),
                        dylib("/usr/lib/libresolv.9.dylib", "0.0.0", "0.0.0"),
                        dylib(
                            "/System/Library/Frameworks/CoreFoundation.framework/Versions/A/CoreFoundation",
                            "0.0.0",
                            "0.0.0",
                        ),
                        dylib(
                            "/System/Library/Frameworks/Security.framework/Versions/A/Security",
                            "0.0.0",
                            "0.0.0",
                        ),
                    ],
                    lc_load_weak_dylib: Vec::new(),
                    lc_reexport_dylib: Vec::new(),
                    lc_load_upward_dylib: Vec::new(),
                    lc_rpath: Vec::new(),
                }],
            }),
        }
    }

    #[test]
    fn a_dylib_version_must_match() {
        let set = one(r#"
format = "macho"
cputype = ["CPU_TYPE_ARM64"]
filetype = "MH_EXECUTE"
LC_LOAD_DYLIB = [{ name = "/usr/lib/libSystem.B.dylib", compatibility_version = "0.0.0", current_version = "0.0.0", why = "macOS has no static libc." }]
"#);
        let mut snap = mach_snap(&["CPU_TYPE_ARM64"]);
        {
            let Records::Macho(mach) = &mut snap.records else {
                panic!("macho");
            };
            mach.slices[0].lc_load_dylib.push(dylib(
                "/usr/lib/libSystem.B.dylib",
                "0.0.0",
                "1351.0.0",
            ));
        }
        let miss = set.misses("bin/tool", &snap).unwrap();
        assert!(
            miss.iter()
                .any(|line| line.contains("current_version") && line.contains("1351.0.0")),
            "{miss:?}"
        );
        {
            let Records::Macho(mach) = &mut snap.records else {
                panic!("macho");
            };
            mach.slices[0].lc_load_dylib[0].current_version = "0.0.0".into();
        }
        assert!(set.misses("bin/tool", &snap).unwrap().is_empty());
    }

    #[test]
    fn one_version_field_needs_the_other() {
        let err = parse_set(&[(
            "bin/tool",
            r#"
format = "macho"
cputype = ["CPU_TYPE_ARM64"]
filetype = "MH_EXECUTE"
LC_LOAD_DYLIB = [{ name = "/usr/lib/libSystem.B.dylib", current_version = "0.0.0", why = "macOS has no static libc." }]
"#,
        )])
        .unwrap_err();
        assert!(err.contains("compatibility_version"), "{err}");
    }

    #[test]
    fn an_elf_grant_rejects_a_macho_version() {
        let err = parse_set(&[(
            "bin/tool",
            r#"
format = "elf"
DT_NEEDED = [{ name = "libc.so.6", compatibility_version = "0.0.0", current_version = "0.0.0", why = "The C library." }]
"#,
        )])
        .unwrap_err();
        assert!(err.contains("unknown field compatibility_version"), "{err}");
    }

    #[test]
    fn header_flags_must_be_the_set_bits() {
        let set = one(r#"
format = "macho"
cputype = ["CPU_TYPE_ARM64"]
filetype = "MH_EXECUTE"
flags = ["MH_DYLDLINK", "MH_PIE"]
MH_PIE = true
LC_LOAD_DYLIB = []
"#);
        let mut snap = mach_snap(&["CPU_TYPE_ARM64"]);
        let Records::Macho(mach) = &mut snap.records else {
            panic!("macho");
        };
        mach.slices[0].mh_pie = true;
        mach.slices[0].flags = vec!["MH_DYLDLINK".into(), "MH_PIE".into(), "MH_TWOLEVEL".into()];
        let miss = set.misses("bin/tool", &snap).unwrap();
        assert!(
            miss.iter()
                .any(|line| line.starts_with("flags:") && line.contains("MH_TWOLEVEL")),
            "{miss:?}"
        );
    }

    #[test]
    fn pie_must_agree_with_flags() {
        let err = parse_set(&[(
            "bin/tool",
            "format = \"macho\"\ncputype = [\"CPU_TYPE_ARM64\"]\nfiletype = \"MH_EXECUTE\"\nflags = [\"MH_DYLDLINK\"]\nMH_PIE = true\nLC_LOAD_DYLIB = []\n",
        )])
        .unwrap_err();
        assert!(err.contains("MH_PIE does not match flags"), "{err}");
    }

    #[test]
    fn cpusubtype_must_match() {
        let set = one(r#"
format = "macho"
cputype = ["CPU_TYPE_ARM64"]
cpusubtype = ["CPU_SUBTYPE_ARM64_ALL"]
filetype = "MH_EXECUTE"
LC_LOAD_DYLIB = []
"#);
        let mut snap = mach_snap(&["CPU_TYPE_ARM64"]);
        let Records::Macho(mach) = &mut snap.records else {
            panic!("macho");
        };
        mach.cpusubtype = vec!["CPU_SUBTYPE_ARM64E"];
        let miss = set.misses("bin/tool", &snap).unwrap();
        assert!(
            miss.iter()
                .any(|line| line.starts_with("cpusubtype:") && line.contains("CPU_SUBTYPE_ARM64E")),
            "{miss:?}"
        );
    }

    #[test]
    fn a_weak_load_is_a_miss() {
        let set = devlayer().unwrap();
        let mut snap = cli_snap();
        let Records::Macho(mach) = &mut snap.records else {
            panic!("macho");
        };
        mach.slices[0]
            .lc_load_weak_dylib
            .push(dylib("/usr/lib/libz.1.dylib", "0.0.0", "0.0.0"));
        let miss = set.misses("bin/devlayer", &snap).unwrap();
        assert!(
            miss.iter()
                .any(|line| line.starts_with("LC_LOAD_WEAK_DYLIB:") && line.contains("libz")),
            "{miss:?}"
        );
    }

    #[test]
    fn a_delay_name_stays_out_of_the_import_list() {
        let set = one(r#"
format = "pe"
IMAGE_IMPORT_DESCRIPTOR = [{ name = "KERNEL32.dll", why = "The Windows base library." }]
"#);
        let good = pe_snap(&["KERNEL32.dll"], &[]);
        assert!(set.misses("bin/tool", &good).unwrap().is_empty());
        let late = pe_snap(&["KERNEL32.dll"], &["libiconv.dll"]);
        let miss = set.misses("bin/tool", &late).unwrap();
        assert!(
            miss.iter()
                .any(|line| line.contains("ImgDelayDescr") && line.contains("libiconv.dll")),
            "{miss:?}"
        );
    }

    fn pe_snap(needed: &[&str], delay: &[&str]) -> FileSnap {
        FileSnap {
            records: Records::Pe(PeSnap {
                image_import_descriptor: needed.iter().map(|name| (*name).to_string()).collect(),
                img_delay_descr: delay.iter().map(|name| (*name).to_string()).collect(),
                dynamic_base: false,
            }),
        }
    }
}
