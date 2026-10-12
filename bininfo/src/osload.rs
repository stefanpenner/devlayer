/*
---
why: This file rejects a host library in a shipped file.
uses: bininfo
how: bininfo --policy calls check_path.
The hermetic claim stays a separate check.
---
*/

//! One policy file describes one binary.
//! A binary with no policy fails.
//! This file does not change the hermetic claim.

use crate::elf::header::is_elf;
use crate::macho::header::{is_macho, Image};
use crate::macho::load::{Commands, DylibCmd, Slice as MachSlice};
use crate::pe::header::is_pe;
use crate::policy::{ElfSnap, FileSnap, MachoDylib, MachoSlice, MachoSnap, PeSnap, Records, Set};
use crate::{open, Binary};
use std::fs;
use std::path::Path;

const ERR_CAP: usize = 32;

/// True when the bytes are ELF, Mach-O, or PE.
pub fn looks_like(bytes: &[u8]) -> bool {
    is_elf(bytes) || is_macho(bytes) || is_pe(bytes)
}

/// Judge one binary against the policy for `rel`.
pub fn judge(rel: &str, file: &Binary, set: &Set) -> Result<(), String> {
    let snap = file_snap(file)?;
    let Some(snap) = snap else {
        return Ok(());
    };
    let misses = set.misses(rel, &snap)?;
    if misses.is_empty() {
        return Ok(());
    }
    Err(misses.join("\n"))
}

/// Check one file or every file under a directory.
pub fn check_path(path: &Path, policy: &Set) -> Result<(), String> {
    let meta = fs::symlink_metadata(path)
        .map_err(|err| format!("cannot read {}: {err}", path.display()))?;
    if meta.is_dir() {
        let root = path
            .canonicalize()
            .map_err(|err| format!("cannot read {}: {err}", path.display()))?;
        let mut errs = Vec::new();
        walk(&root, &root, policy, &mut errs)?;
        return finish(&mut errs);
    }
    check_file(path, &slash_path(path), policy)
}

fn walk(root: &Path, dir: &Path, policy: &Set, errs: &mut Vec<String>) -> Result<(), String> {
    if errs.len() >= ERR_CAP {
        return Ok(());
    }
    let entries =
        fs::read_dir(dir).map_err(|err| format!("cannot read {}: {err}", dir.display()))?;
    for entry in entries {
        if errs.len() >= ERR_CAP {
            return Ok(());
        }
        let entry = entry.map_err(|err| format!("cannot read {}: {err}", dir.display()))?;
        let path = entry.path();
        let kind = entry
            .file_type()
            .map_err(|err| format!("cannot read {}: {err}", path.display()))?;
        if kind.is_dir() {
            walk(root, &path, policy, errs)?;
            continue;
        }
        if kind.is_symlink() {
            match fs::read_link(&path) {
                Ok(link) if link_in_tree(root, &path, &link) => {}
                Ok(_) => push(
                    errs,
                    format!("bininfo: symlink {} leaves the tree", path.display()),
                ),
                Err(err) => push(errs, format!("bininfo: symlink {}: {err}", path.display())),
            }
            continue;
        }
        if kind.is_file() {
            if let Err(err) = check_file(&path, &rel_path(root, &path), policy) {
                for line in err.lines() {
                    push(errs, line.to_string());
                }
            }
        }
    }
    Ok(())
}

fn check_file(path: &Path, rel: &str, policy: &Set) -> Result<(), String> {
    let bytes = fs::read(path).map_err(|err| format!("cannot read {}: {err}", path.display()))?;
    if !looks_like(&bytes) {
        return Ok(());
    }
    let file = open(&bytes, false).map_err(|err| format!("{}: bininfo: {err}", path.display()))?;
    judge(rel, &file, policy).map_err(|err| format!("{}: bininfo: {err}", path.display()))
}

fn rel_path(root: &Path, path: &Path) -> String {
    match path.strip_prefix(root) {
        Ok(rel) => slash_path(rel),
        Err(_) => slash_path(path),
    }
}

fn slash_path(path: &Path) -> String {
    path.to_string_lossy().replace('\\', "/")
}

fn file_snap(file: &Binary) -> Result<Option<FileSnap>, String> {
    match file {
        Binary::Elf { load, .. } => Ok(Some(FileSnap {
            records: Records::Elf(ElfSnap {
                pt_interp: load.interp.clone(),
                dt_needed: load.needed.clone(),
                dt_runpath: load.runpath.clone(),
                dt_rpath: load.rpath.clone(),
                df_1_pie: load.pie,
            }),
        })),
        Binary::Pe { load, .. } => Ok(Some(FileSnap {
            records: Records::Pe(PeSnap {
                image_import_descriptor: load.needed.clone(),
                img_delay_descr: load.delay.clone(),
                dynamic_base: load.pie,
            }),
        })),
        Binary::Macho {
            image,
            commands,
            slices,
        } => macho_snap(image, commands, slices),
        Binary::Zip(_) | Binary::Dmg(_) | Binary::Tiff(_) => Ok(None),
    }
}

fn macho_snap(
    image: &Image,
    commands: &Commands,
    slices: &[MachSlice],
) -> Result<Option<FileSnap>, String> {
    match image {
        Image::Fat(_) => {
            if slices.is_empty() || slices.iter().any(|slice| slice.thin.is_none()) {
                return Err("unchecked mach-o".into());
            }
            let cputype = slices
                .iter()
                .map(|slice| crate::macho::cpu_const(slice.cputype))
                .collect();
            let cpusubtype = slices
                .iter()
                .map(|slice| crate::macho::subtype_const(slice.cputype, slice.cpusubtype))
                .collect();
            let filetype = slices
                .iter()
                .map(|slice| {
                    crate::macho::file_type_name(slice.thin.as_ref().unwrap().filetype).to_string()
                })
                .collect();
            let rows = slices
                .iter()
                .map(|slice| {
                    let flags = slice.thin.as_ref().unwrap().flags;
                    load_slice(&slice.commands.load, flags)
                })
                .collect();
            Ok(Some(FileSnap {
                records: Records::Macho(MachoSnap {
                    cputype,
                    cpusubtype,
                    filetype,
                    slices: rows,
                }),
            }))
        }
        Image::Thin(thin) => Ok(Some(FileSnap {
            records: Records::Macho(MachoSnap {
                cputype: vec![crate::macho::cpu_const(thin.cputype)],
                cpusubtype: vec![crate::macho::subtype_const(thin.cputype, thin.cpusubtype)],
                filetype: vec![crate::macho::file_type_name(thin.filetype).to_string()],
                slices: vec![load_slice(&commands.load, thin.flags)],
            }),
        })),
    }
}

fn load_slice(load: &crate::macho::load::Load, flags: u32) -> MachoSlice {
    MachoSlice {
        mh_pie: load.pie,
        flags: crate::macho::flag_names(flags),
        lc_load_dylinker: load.interp.clone(),
        lc_load_dylib: dylibs(load, DylibCmd::Load),
        lc_load_weak_dylib: dylibs(load, DylibCmd::Weak),
        lc_reexport_dylib: dylibs(load, DylibCmd::Reexport),
        lc_load_upward_dylib: dylibs(load, DylibCmd::Upward),
        lc_rpath: load.rpath.clone(),
    }
}

fn dylibs(load: &crate::macho::load::Load, cmd: DylibCmd) -> Vec<MachoDylib> {
    load.needed
        .iter()
        .filter(|row| row.cmd == cmd)
        .map(|row| MachoDylib {
            name: row.name.clone(),
            compatibility_version: row.compat.clone(),
            current_version: row.current.clone(),
        })
        .collect()
}

fn link_in_tree(root: &Path, path: &Path, link: &Path) -> bool {
    if let Ok(target) = path.canonicalize() {
        return target.starts_with(root);
    }
    let joined = if link.is_absolute() {
        link.to_path_buf()
    } else {
        let Some(parent) = path.parent() else {
            return false;
        };
        parent.join(link)
    };
    let mut norm = std::path::PathBuf::new();
    for part in joined.components() {
        match part {
            std::path::Component::CurDir => {}
            std::path::Component::ParentDir => {
                if !norm.pop() {
                    return false;
                }
            }
            other => norm.push(other),
        }
    }
    norm.starts_with(root)
}

fn push(errs: &mut Vec<String>, line: String) {
    if errs.len() < ERR_CAP {
        errs.push(line);
    }
}

fn finish(errs: &mut Vec<String>) -> Result<(), String> {
    if errs.is_empty() {
        return Ok(());
    }
    if errs.len() == ERR_CAP {
        errs.push("bininfo: more host libraries".into());
    }
    Err(errs.join("\n"))
}

#[cfg(test)]
mod tests {
    use super::check_path;
    use crate::policy::devlayer;
    use std::fs;

    #[cfg(unix)]
    #[test]
    fn dangling_absolute_symlink_leaves_the_tree() {
        let dir = std::env::temp_dir().join(format!("os-load-dangling-{}", std::process::id()));
        let _ = fs::remove_dir_all(&dir);
        fs::create_dir(&dir).unwrap();
        std::os::unix::fs::symlink("/usr/bin/htop", dir.join("htop")).unwrap();
        let policy = devlayer().unwrap();
        let err = check_path(&dir, &policy).unwrap_err();
        assert!(err.contains("leaves the tree"), "{err}");
        assert!(err.contains("htop"), "{err}");
        let _ = fs::remove_dir_all(&dir);
    }
}
