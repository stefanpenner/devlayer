/*
---
why: This file reads PE versions and import rows.
uses: bininfo
how: bininfo calls this file to count imports and name the entry.
This file does not print the page.
---
*/

//! PE versions and import rows.
//! `needed` stays the DLL name list. This module reads the thunks.
//! This module does not print.

use super::header::{self, Class, Header};
use super::load::{self, Sec};
use crate::claim::{CpuFloor, Version};

const IMPORT_CAP: u32 = 256;
const THUNK_CAP: u32 = 4096;
const EXPORT_CAP: u32 = 4096;

/// One import thunk. An ordinal row has no name.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Import {
    pub dll: String,
    pub name: Option<String>,
    pub hint: Option<u16>,
    pub ordinal: Option<u32>,
}

/// Version and symbol counts stored in the PE header.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Symbols {
    pub os: Option<(u16, u16)>,
    pub subsystem: Option<(u16, u16)>,
    pub import_count: u32,
    pub ordinal_count: u32,
    pub export_count: u32,
    pub coff_symbols: u32,
    pub imports: Vec<Import>,
    pub entry_name: Option<String>,
    pub entry_note: &'static str,
    pub rows: bool,
}

impl Symbols {
    #[cfg(test)]
    pub fn empty() -> Self {
        Self {
            os: None,
            subsystem: None,
            import_count: 0,
            ordinal_count: 0,
            export_count: 0,
            coff_symbols: 0,
            imports: Vec::new(),
            entry_name: None,
            entry_note: "[IMAGE_EXPORT_DIRECTORY] No export RVA equals the entry.",
            rows: false,
        }
    }
}

pub fn pair_text(pair: (u16, u16)) -> String {
    format!("{}.{}", pair.0, pair.1)
}

pub fn read(bytes: &[u8], header: &Header, rows: bool) -> Symbols {
    let os = version_at(bytes, header, 40);
    let subsystem = version_at(bytes, header, 48);
    let sections = load::sections(bytes, header);
    let (import_count, ordinal_count, imports) = imports(bytes, header, &sections, rows);
    let (export_count, entry_name) = exports(bytes, header, &sections);
    let entry_note = if entry_name.is_some() {
        "[IMAGE_EXPORT_DIRECTORY] Export whose RVA equals the entry."
    } else {
        "[IMAGE_EXPORT_DIRECTORY] No export RVA equals the entry."
    };
    Symbols {
        os,
        subsystem,
        import_count,
        ordinal_count,
        export_count,
        coff_symbols: header.nsyms,
        imports,
        entry_name,
        entry_note,
        rows,
    }
}

pub fn floors(header: &Header, symbols: &Symbols) -> Vec<CpuFloor> {
    let word = super::words::machine_name(header.machine).unwrap_or("");
    let minos = symbols.os.map(|(major, minor)| Version {
        major: u32::from(major),
        minor: u32::from(minor),
        patch: 0,
    });
    let os_family = if minos.is_some() { "windows" } else { "" };
    if word.is_empty() && minos.is_none() {
        return Vec::new();
    }
    vec![CpuFloor {
        word,
        arm64e: false,
        minos,
        os_family,
    }]
}

fn version_at(bytes: &[u8], header: &Header, rel: u16) -> Option<(u16, u16)> {
    if header.class == Class::Coff || rel.saturating_add(4) > header.opt_size {
        return None;
    }
    let base = header.opt_off as usize;
    let major = header::u16_at(bytes, base + rel as usize)?;
    let minor = header::u16_at(bytes, base + rel as usize + 2)?;
    Some((major, minor))
}

fn imports(bytes: &[u8], header: &Header, sections: &[Sec], rows: bool) -> (u32, u32, Vec<Import>) {
    let Some((rva, size)) = load::data_dir(bytes, header, 1) else {
        return (0, 0, Vec::new());
    };
    let Some(start) = load::rva_off(sections, rva) else {
        return (0, 0, Vec::new());
    };
    let byte_cap = if size == 0 {
        IMPORT_CAP as usize * 20
    } else {
        size as usize
    };
    let wide = header.class != Class::Pe32;
    let mut import_count = 0u32;
    let mut ordinal_count = 0u32;
    let mut found = Vec::new();
    let mut at = start;
    let mut n = 0u32;
    while n < IMPORT_CAP && import_count < THUNK_CAP && at.saturating_sub(start) + 20 <= byte_cap {
        let Some(name_rva) = header::u32_at(bytes, at + 12) else {
            break;
        };
        let oft = header::u32_at(bytes, at).unwrap_or(0);
        let stamp = header::u32_at(bytes, at + 4).unwrap_or(0);
        let fwd = header::u32_at(bytes, at + 8).unwrap_or(0);
        let ft = header::u32_at(bytes, at + 16).unwrap_or(0);
        if oft == 0 && stamp == 0 && fwd == 0 && name_rva == 0 && ft == 0 {
            break;
        }
        let dll = load::rva_off(sections, name_rva)
            .map(|off| load::read_text(bytes, off))
            .unwrap_or_default();
        let ilt = if oft != 0 { oft } else { ft };
        if !dll.is_empty() && ilt != 0 {
            walk_thunks(
                bytes,
                sections,
                ilt,
                wide,
                &dll,
                rows,
                &mut import_count,
                &mut ordinal_count,
                &mut found,
            );
        }
        at += 20;
        n += 1;
    }
    (import_count, ordinal_count, found)
}

fn walk_thunks(
    bytes: &[u8],
    sections: &[Sec],
    ilt: u32,
    wide: bool,
    dll: &str,
    rows: bool,
    import_count: &mut u32,
    ordinal_count: &mut u32,
    found: &mut Vec<Import>,
) {
    let Some(start) = load::rva_off(sections, ilt) else {
        return;
    };
    let step = if wide { 8 } else { 4 };
    let mut at = start;
    while *import_count < THUNK_CAP {
        if wide {
            let Some(word) = header::u64_at(bytes, at) else {
                break;
            };
            if word == 0 {
                break;
            }
            if word & 0x8000_0000_0000_0000 != 0 {
                push_ordinal(
                    dll,
                    (word & 0xffff) as u32,
                    rows,
                    import_count,
                    ordinal_count,
                    found,
                );
            } else {
                push_name(bytes, sections, word as u32, dll, rows, import_count, found);
            }
        } else {
            let Some(word) = header::u32_at(bytes, at) else {
                break;
            };
            if word == 0 {
                break;
            }
            if word & 0x8000_0000 != 0 {
                push_ordinal(dll, word & 0xffff, rows, import_count, ordinal_count, found);
            } else {
                push_name(bytes, sections, word, dll, rows, import_count, found);
            }
        }
        at += step;
    }
}

fn push_ordinal(
    dll: &str,
    ordinal: u32,
    rows: bool,
    import_count: &mut u32,
    ordinal_count: &mut u32,
    found: &mut Vec<Import>,
) {
    *import_count = import_count.saturating_add(1);
    *ordinal_count = ordinal_count.saturating_add(1);
    if rows {
        found.push(Import {
            dll: dll.to_string(),
            name: None,
            hint: None,
            ordinal: Some(ordinal),
        });
    }
}

fn push_name(
    bytes: &[u8],
    sections: &[Sec],
    rva: u32,
    dll: &str,
    rows: bool,
    import_count: &mut u32,
    found: &mut Vec<Import>,
) {
    *import_count = import_count.saturating_add(1);
    if !rows {
        return;
    }
    let Some(off) = load::rva_off(sections, rva) else {
        found.push(Import {
            dll: dll.to_string(),
            name: None,
            hint: None,
            ordinal: None,
        });
        return;
    };
    let hint = header::u16_at(bytes, off);
    let name = load::read_text(bytes, off.saturating_add(2));
    found.push(Import {
        dll: dll.to_string(),
        name: if name.is_empty() { None } else { Some(name) },
        hint,
        ordinal: None,
    });
}

fn exports(bytes: &[u8], header: &Header, sections: &[Sec]) -> (u32, Option<String>) {
    let Some((rva, _)) = load::data_dir(bytes, header, 0) else {
        return (0, None);
    };
    let Some(dir) = load::rva_off(sections, rva) else {
        return (0, None);
    };
    let nfuncs = header::u32_at(bytes, dir + 20).unwrap_or(0);
    let nnames = header::u32_at(bytes, dir + 24).unwrap_or(0);
    let funcs = header::u32_at(bytes, dir + 28).unwrap_or(0);
    let names = header::u32_at(bytes, dir + 32).unwrap_or(0);
    let ords = header::u32_at(bytes, dir + 36).unwrap_or(0);
    let mut entry_name = None;
    let walk = nnames.min(EXPORT_CAP);
    for index in 0..walk {
        if entry_name.is_some() {
            break;
        }
        let Some(name_rva) = read_rva(bytes, sections, names, index * 4) else {
            break;
        };
        let Some(ord_at) = load::rva_off(sections, ords.saturating_add(index * 2)) else {
            break;
        };
        let Some(ord) = header::u16_at(bytes, ord_at) else {
            break;
        };
        if u32::from(ord) >= nfuncs {
            continue;
        }
        let Some(func) = read_rva(bytes, sections, funcs, u32::from(ord) * 4) else {
            continue;
        };
        if func == header.entry {
            if let Some(off) = load::rva_off(sections, name_rva) {
                let name = load::read_text(bytes, off);
                if !name.is_empty() {
                    entry_name = Some(name);
                }
            }
        }
    }
    (nnames, entry_name)
}

fn read_rva(bytes: &[u8], sections: &[Sec], base: u32, add: u32) -> Option<u32> {
    let at = load::rva_off(sections, base.checked_add(add)?)?;
    header::u32_at(bytes, at)
}

#[cfg(test)]
mod tests {
    use super::super::header;
    use super::super::page;
    use super::*;

    fn put_u16(buf: &mut [u8], at: usize, v: u16) {
        buf[at..at + 2].copy_from_slice(&v.to_le_bytes());
    }
    fn put_u32(buf: &mut [u8], at: usize, v: u32) {
        buf[at..at + 4].copy_from_slice(&v.to_le_bytes());
    }
    fn put_u64(buf: &mut [u8], at: usize, v: u64) {
        buf[at..at + 8].copy_from_slice(&v.to_le_bytes());
    }

    /// PE32+ with one named import, one ordinal import, and an export that misses the entry.
    fn image() -> Vec<u8> {
        let opt_size = 240u16;
        let lfanew = 0x40u32;
        let opt = (lfanew + 24) as usize;
        let section = opt + opt_size as usize;
        let body = section + 40;
        let mut blob = vec![0u8; body + 0x300];
        blob[0] = b'M';
        blob[1] = b'Z';
        put_u32(&mut blob, 0x3c, lfanew);
        blob[lfanew as usize..lfanew as usize + 4].copy_from_slice(b"PE\0\0");
        let coff = (lfanew + 4) as usize;
        put_u16(&mut blob, coff, 0x8664);
        put_u16(&mut blob, coff + 2, 1);
        put_u16(&mut blob, coff + 16, opt_size);
        put_u16(&mut blob, coff + 18, 0x22);
        put_u16(&mut blob, opt, 0x20b);
        put_u32(&mut blob, opt + 16, 0x2000);
        put_u16(&mut blob, opt + 40, 6);
        put_u16(&mut blob, opt + 42, 0);
        put_u16(&mut blob, opt + 48, 6);
        put_u16(&mut blob, opt + 50, 0);
        put_u32(&mut blob, opt + 60, body as u32);
        put_u16(&mut blob, opt + 68, 3);
        put_u32(&mut blob, opt + 108, 16);
        put_u32(&mut blob, opt + 112, 0x1100);
        put_u32(&mut blob, opt + 116, 40);
        put_u32(&mut blob, opt + 120, 0x1000);
        put_u32(&mut blob, opt + 124, 80);
        blob[section..section + 6].copy_from_slice(b".rdata");
        put_u32(&mut blob, section + 8, 0x300);
        put_u32(&mut blob, section + 12, 0x1000);
        put_u32(&mut blob, section + 16, 0x300);
        put_u32(&mut blob, section + 20, body as u32);

        // Two import descriptors at RVA 0x1000, then a null descriptor.
        let desc = body;
        put_u32(&mut blob, desc, 0x1060);
        put_u32(&mut blob, desc + 12, 0x1090);
        put_u32(&mut blob, desc + 16, 0x1060);
        put_u32(&mut blob, desc + 20, 0x1078);
        put_u32(&mut blob, desc + 32, 0x10a0);
        put_u32(&mut blob, desc + 36, 0x1078);

        // kernel32 thunks: name RVA, then end. user32 thunks: ordinal 23, then end.
        put_u64(&mut blob, body + 0x60, 0x10b0);
        put_u64(&mut blob, body + 0x78, 0x8000_0000_0000_0017);

        write_text(&mut blob, body + 0x90, "kernel32.dll");
        write_text(&mut blob, body + 0xa0, "user32.dll");
        put_u16(&mut blob, body + 0xb0, 352);
        write_text(&mut blob, body + 0xb2, "ExitProcess");

        // Export directory at RVA 0x1100. NumberOfNames is 1. Function RVA is not the entry.
        let exp = body + 0x100;
        put_u32(&mut blob, exp + 20, 1);
        put_u32(&mut blob, exp + 24, 1);
        put_u32(&mut blob, exp + 28, 0x1140);
        put_u32(&mut blob, exp + 32, 0x1144);
        put_u32(&mut blob, exp + 36, 0x1148);
        put_u32(&mut blob, body + 0x140, 0x3000);
        put_u32(&mut blob, body + 0x144, 0x1150);
        put_u16(&mut blob, body + 0x148, 0);
        write_text(&mut blob, body + 0x150, "Start");
        blob
    }

    fn write_text(buf: &mut [u8], at: usize, text: &str) {
        buf[at..at + text.len()].copy_from_slice(text.as_bytes());
    }

    #[test]
    fn versions_named_import_and_ordinal_keep_directory_fields() {
        let bytes = image();
        let header = header::parse(&bytes).unwrap();
        let symbols = read(&bytes, &header, true);
        assert_eq!(symbols.os, Some((6, 0)));
        assert_eq!(symbols.subsystem, Some((6, 0)));
        assert_eq!(symbols.import_count, 2);
        assert_eq!(symbols.ordinal_count, 1);
        assert_eq!(symbols.export_count, 1);
        assert_eq!(symbols.coff_symbols, 0);
        assert!(symbols.entry_name.is_none());
        assert_eq!(symbols.imports.len(), 2);
        assert_eq!(symbols.imports[0].dll, "kernel32.dll");
        assert_eq!(symbols.imports[0].name.as_deref(), Some("ExitProcess"));
        assert_eq!(symbols.imports[0].hint, Some(352));
        assert_eq!(symbols.imports[1].dll, "user32.dll");
        assert_eq!(symbols.imports[1].ordinal, Some(23));
        assert!(symbols.imports[1].name.is_none());

        let load = load::read(&bytes, &header);
        let text = page::render_with("a.exe", &header, &load, &symbols, false);
        assert!(text.contains("os_version: 6.0"), "{text}");
        assert!(text.contains("subsystem_version: 6.0"), "{text}");
        assert!(text.contains("dll: kernel32.dll"), "{text}");
        assert!(text.contains("name: ExitProcess"), "{text}");
        assert!(text.contains("hint: 352"), "{text}");
        assert!(text.contains("ordinal: 23"), "{text}");
        assert!(text.contains("entry_name: null"), "{text}");
        assert!(!text.contains("says:"), "{text}");
    }

    #[test]
    fn a_matching_export_rva_names_the_entry() {
        let mut bytes = image();
        let header = header::parse(&bytes).unwrap();
        let body = header.section_off as usize + 40;
        put_u32(&mut bytes, body + 0x140, header.entry);
        let header = header::parse(&bytes).unwrap();
        let symbols = read(&bytes, &header, false);
        assert_eq!(symbols.entry_name.as_deref(), Some("Start"));
    }

    #[test]
    fn hostile_import_and_export_counts_do_not_panic() {
        let mut bytes = image();
        let header = header::parse(&bytes).unwrap();
        let body = header.section_off as usize + 40;
        put_u32(&mut bytes, body + 0x100 + 24, u32::MAX);
        put_u32(&mut bytes, header.opt_off as usize + 124, u32::MAX);
        let _ = read(&bytes, &header, true);
        let _ = read(&[], &header, true);
        let short = header::parse(&bytes[..80.min(bytes.len())]);
        if let Ok(header) = short {
            let _ = read(&bytes[..80.min(bytes.len())], &header, true);
        }
    }
}
