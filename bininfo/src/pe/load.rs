//! How a PE file starts, and which DLLs it must open.
//! This module reads the import directory and the delay-import directory. It does not print them.

use super::header::{self, Class, Header};
pub(crate) use crate::kind::name as kind_name;
pub use crate::kind::Kind;

const EXEC: u16 = 0x0002;
const DLL: u16 = 0x2000;
const RELOCS_STRIPPED: u16 = 0x0001;
const DYNAMIC_BASE: u16 = 0x0040;
const NAME_CAP: usize = 4096;
const SECTION_CAP: u16 = 96;
const IMPORT_CAP: u32 = 256;
const DELAY_DIR: u32 = 13;
const DELAY_ATTR_RVA: u32 = 1;
const DELAY_CAP: u32 = 256;
const DELAY_BYTES: usize = 32;

/// Facts that match the ELF load map. Windows stores no loader path.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Load {
    pub kind: Kind,
    pub interp: Option<String>, // Always None. The file does not name a loader.
    pub needed: Vec<String>,    // DLL names the loader opens.
    pub runpath: Vec<String>,   // Always empty. PE has no search-path record.
    pub rpath: Vec<String>,     // Always empty. PE has no search-path record.
    pub delay: Vec<String>,     // Delay-load DLL names. The loader opens them on the first call.
    pub pie: bool,              // DYNAMIC_BASE bit, when relocations remain.
}

pub fn read(bytes: &[u8], header: &Header) -> Load {
    let pie = header.dll_characteristics & DYNAMIC_BASE != 0
        && header.characteristics & RELOCS_STRIPPED == 0;
    let needed = imports(bytes, header);
    let delay = delay_names(bytes, header);
    Load {
        kind: classify(header, !needed.is_empty(), pie),
        interp: None,
        needed,
        runpath: Vec::new(),
        rpath: Vec::new(),
        delay,
        pie,
    }
}

pub(crate) fn kind_note(kind: Kind) -> &'static str {
    match kind {
        Kind::Static => "EXE and no imported DLL.",
        Kind::Dynamic => "This file imports a DLL and DYNAMIC_BASE is clear.",
        Kind::DynamicPie => "This file imports a DLL and DYNAMIC_BASE is set.",
        Kind::StaticPie => "EXE, no imported DLL, and DYNAMIC_BASE is set.",
        Kind::Shared => "DLL. Other files open this file.",
        Kind::Rel => "COFF object. The loader does not run this file.",
        Kind::Other => "This file is outside the four start kinds.",
    }
}

fn classify(header: &Header, imports: bool, pie: bool) -> Kind {
    if header.class == Class::Coff {
        return Kind::Rel;
    }
    let exe = header.characteristics & EXEC != 0;
    let dll = header.characteristics & DLL != 0;
    if dll {
        return Kind::Shared;
    }
    if !exe {
        return Kind::Other;
    }
    match (imports, pie) {
        (false, false) => Kind::Static,
        (false, true) => Kind::StaticPie,
        (true, false) => Kind::Dynamic,
        (true, true) => Kind::DynamicPie,
    }
}

pub(crate) struct Sec {
    pub(crate) va: u32,
    pub(crate) raw: u32,
    pub(crate) ptr: u32,
}

fn imports(bytes: &[u8], header: &Header) -> Vec<String> {
    let Some((rva, size)) = data_dir(bytes, header, 1) else {
        return Vec::new();
    };
    let sections = sections(bytes, header);
    let Some(start) = rva_off(&sections, rva) else {
        return Vec::new();
    };
    let byte_cap = if size == 0 {
        IMPORT_CAP as usize * 20
    } else {
        size as usize
    };
    let mut names = Vec::new();
    let mut at = start;
    let mut n = 0u32;
    while n < IMPORT_CAP && at.saturating_sub(start) + 20 <= byte_cap {
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
        if let Some(off) = rva_off(&sections, name_rva) {
            let name = read_text(bytes, off);
            if !name.is_empty() {
                names.push(name);
            }
        }
        at += 20;
        n += 1;
    }
    names
}

fn delay_names(bytes: &[u8], header: &Header) -> Vec<String> {
    let Some((rva, size)) = data_dir(bytes, header, DELAY_DIR) else {
        return Vec::new();
    };
    let sections = sections(bytes, header);
    let Some(start) = rva_off(&sections, rva) else {
        return Vec::new();
    };
    let byte_cap = if size == 0 {
        DELAY_CAP as usize * DELAY_BYTES
    } else {
        size as usize
    };
    let Some(end) = start.checked_add(byte_cap) else {
        return Vec::new();
    };
    let mut names = Vec::new();
    let mut at = start;
    let mut n = 0u32;
    while n < DELAY_CAP {
        let Some(next) = at.checked_add(DELAY_BYTES) else {
            break;
        };
        if next > end {
            break;
        }
        let Some(words) = desc_words(bytes, at) else {
            break;
        };
        if words.iter().all(|word| *word == 0) {
            break;
        }
        let name_rva = if words[0] & DELAY_ATTR_RVA != 0 {
            words[1]
        } else {
            va_to_rva(words[1], header.image_base)
        };
        if let Some(off) = rva_off(&sections, name_rva) {
            let name = read_text(bytes, off);
            if !name.is_empty() {
                names.push(name);
            }
        }
        at = next;
        n += 1;
    }
    names
}

pub(crate) fn data_dir(bytes: &[u8], header: &Header, index: u32) -> Option<(u32, u32)> {
    let (nrva_rel, dir_rel) = match header.class {
        Class::Pe32 => (92u32, 96u32),
        Class::Pe32Plus => (108, 112),
        Class::Coff => return None,
    };
    let entry = dir_rel.checked_add(index.checked_mul(8)?)?;
    if (header.opt_size as u32) < entry.checked_add(8)? {
        return None;
    }
    let opt = header.opt_off as usize;
    let nrva = header::u32_at(bytes, opt.checked_add(nrva_rel as usize)?)?;
    if nrva <= index {
        return None;
    }
    let at = opt.checked_add(entry as usize)?;
    let rva = header::u32_at(bytes, at)?;
    let size = header::u32_at(bytes, at.checked_add(4)?)?;
    if rva == 0 {
        return None;
    }
    Some((rva, size))
}

fn desc_words(bytes: &[u8], at: usize) -> Option<[u32; 8]> {
    let mut words = [0u32; 8];
    for (index, word) in words.iter_mut().enumerate() {
        let pos = at.checked_add(index * 4)?;
        *word = header::u32_at(bytes, pos)?;
    }
    Some(words)
}

fn va_to_rva(va: u32, image_base: u64) -> u32 {
    let Ok(base) = u32::try_from(image_base) else {
        return 0;
    };
    va.checked_sub(base).unwrap_or(0)
}

pub(crate) fn sections(bytes: &[u8], header: &Header) -> Vec<Sec> {
    let mut at = header.section_off as usize;
    let mut out = Vec::new();
    for _ in 0..header.sections.min(SECTION_CAP) {
        if at + 40 > bytes.len() {
            break;
        }
        out.push(Sec {
            va: header::u32_at(bytes, at + 12).unwrap_or(0),
            raw: header::u32_at(bytes, at + 16).unwrap_or(0),
            ptr: header::u32_at(bytes, at + 20).unwrap_or(0),
        });
        at += 40;
    }
    out
}

pub(crate) fn rva_off(sections: &[Sec], rva: u32) -> Option<usize> {
    sections.iter().find_map(|sec| {
        if rva < sec.va {
            return None;
        }
        let delta = rva - sec.va;
        if delta < sec.raw {
            Some(sec.ptr as usize + delta as usize)
        } else {
            None
        }
    })
}

pub(crate) fn read_text(bytes: &[u8], offset: usize) -> String {
    if offset >= bytes.len() {
        return String::new();
    }
    let take = (bytes.len() - offset).min(NAME_CAP);
    let slice = &bytes[offset..offset + take];
    let end = slice.iter().position(|b| *b == 0).unwrap_or(slice.len());
    String::from_utf8_lossy(&slice[..end]).into_owned()
}

#[cfg(test)]
mod tests {
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

    /// PE32+ with one section and two import names.
    fn image(chars: u16, dllc: u16, names: &[&str]) -> Vec<u8> {
        let opt_size = 128u16;
        let lfanew = 0x40u32;
        let opt = (lfanew + 4 + 20) as usize;
        let section = opt + opt_size as usize;
        let body = section + 40;
        let mut blob = vec![0u8; body + 0x100];
        blob[0] = b'M';
        blob[1] = b'Z';
        put_u32(&mut blob, 0x3c, lfanew);
        blob[lfanew as usize..lfanew as usize + 4].copy_from_slice(b"PE\0\0");
        let coff = (lfanew + 4) as usize;
        put_u16(&mut blob, coff, 0x8664);
        put_u16(&mut blob, coff + 2, 1);
        put_u16(&mut blob, coff + 16, opt_size);
        put_u16(&mut blob, coff + 18, chars);
        put_u16(&mut blob, opt, 0x20b);
        put_u32(&mut blob, opt + 16, 0x1000);
        put_u32(&mut blob, opt + 60, body as u32);
        put_u16(&mut blob, opt + 68, 3);
        put_u16(&mut blob, opt + 70, dllc);
        put_u32(&mut blob, opt + 108, 16);
        if !names.is_empty() {
            put_u32(&mut blob, opt + 112 + 8, 0x1000);
            put_u32(&mut blob, opt + 112 + 12, 40);
        }
        blob[section..section + 6].copy_from_slice(b".rdata");
        put_u32(&mut blob, section + 8, 0x100);
        put_u32(&mut blob, section + 12, 0x1000);
        put_u32(&mut blob, section + 16, 0x100);
        put_u32(&mut blob, section + 20, body as u32);
        if !names.is_empty() {
            let mut name_at = 0x40u32;
            for (i, name) in names.iter().enumerate() {
                let desc = body + i * 20;
                put_u32(&mut blob, desc, 1);
                put_u32(&mut blob, desc + 12, 0x1000 + name_at);
                put_u32(&mut blob, desc + 16, 1);
                let file_at = body + name_at as usize;
                blob[file_at..file_at + name.len()].copy_from_slice(name.as_bytes());
                name_at += name.len() as u32 + 1;
            }
        }
        blob
    }

    /// PE32+ with optional import names and delay-load names.
    /// `rva_form` false stores the old virtual-address delay record.
    fn delay_image(needed: &[&str], delay: &[&str], rva_form: bool) -> Vec<u8> {
        let opt_size = 240u16;
        let lfanew = 0x40u32;
        let opt = (lfanew + 4 + 20) as usize;
        let section = opt + opt_size as usize;
        let body = section + 40;
        let mut blob = vec![0u8; body + 0x200];
        blob[0] = b'M';
        blob[1] = b'Z';
        put_u32(&mut blob, 0x3c, lfanew);
        blob[lfanew as usize..lfanew as usize + 4].copy_from_slice(b"PE\0\0");
        let coff = (lfanew + 4) as usize;
        put_u16(&mut blob, coff, 0x8664);
        put_u16(&mut blob, coff + 2, 1);
        put_u16(&mut blob, coff + 16, opt_size);
        put_u16(&mut blob, coff + 18, EXEC);
        put_u16(&mut blob, opt, 0x20b);
        put_u32(&mut blob, opt + 16, 0x1000);
        let base = if rva_form { 0x1_4000_0000 } else { 0x40_0000 };
        put_u64(&mut blob, opt + 24, base);
        put_u32(&mut blob, opt + 60, body as u32);
        put_u16(&mut blob, opt + 68, 3);
        put_u16(&mut blob, opt + 70, DYNAMIC_BASE);
        put_u32(&mut blob, opt + 108, 16);
        blob[section..section + 6].copy_from_slice(b".rdata");
        put_u32(&mut blob, section + 8, 0x200);
        put_u32(&mut blob, section + 12, 0x1000);
        put_u32(&mut blob, section + 16, 0x200);
        put_u32(&mut blob, section + 20, body as u32);
        let mut name_at = 0u32;
        if !needed.is_empty() {
            put_u32(&mut blob, opt + 120, 0x1000);
            put_u32(&mut blob, opt + 124, (needed.len() * 20) as u32);
        }
        if !delay.is_empty() {
            put_u32(&mut blob, opt + 216, 0x1080);
            put_u32(&mut blob, opt + 220, (delay.len() * 32) as u32);
        }
        for (i, name) in needed.iter().enumerate() {
            let rva = 0x1100 + name_at;
            let desc = body + i * 20;
            put_u32(&mut blob, desc, 1);
            put_u32(&mut blob, desc + 12, rva);
            put_u32(&mut blob, desc + 16, 1);
            let file_at = body + 0x100 + name_at as usize;
            blob[file_at..file_at + name.len()].copy_from_slice(name.as_bytes());
            name_at += name.len() as u32 + 1;
        }
        for (i, name) in delay.iter().enumerate() {
            let rva = 0x1100 + name_at;
            let desc = body + 0x80 + i * 32;
            put_u32(&mut blob, desc, if rva_form { DELAY_ATTR_RVA } else { 0 });
            let stored = if rva_form { rva } else { 0x40_0000 + rva };
            put_u32(&mut blob, desc + 4, stored);
            let file_at = body + 0x100 + name_at as usize;
            blob[file_at..file_at + name.len()].copy_from_slice(name.as_bytes());
            name_at += name.len() as u32 + 1;
        }
        blob
    }

    #[test]
    fn static_exe_has_no_import() {
        let bytes = image(EXEC, 0, &[]);
        let header = header::parse(&bytes).unwrap();
        let load = read(&bytes, &header);
        assert_eq!(load.kind, Kind::Static);
        assert!(load.needed.is_empty());
        assert!(load.runpath.is_empty());
        assert!(load.rpath.is_empty());
        assert!(load.delay.is_empty());
        assert!(!load.pie);
        assert!(load.interp.is_none());
    }

    #[test]
    fn static_pie_sets_dynamic_base() {
        let bytes = image(EXEC, DYNAMIC_BASE, &[]);
        let header = header::parse(&bytes).unwrap();
        let load = read(&bytes, &header);
        assert_eq!(load.kind, Kind::StaticPie);
        assert!(load.pie);
    }

    #[test]
    fn dynamic_pie_names_two_dlls() {
        let bytes = image(EXEC, DYNAMIC_BASE, &["a.dll", "b.dll"]);
        let header = header::parse(&bytes).unwrap();
        let load = read(&bytes, &header);
        assert_eq!(load.kind, Kind::DynamicPie);
        assert_eq!(load.needed, ["a.dll", "b.dll"]);
        assert!(load.pie);
    }

    #[test]
    fn dynamic_without_dynamic_base() {
        let bytes = image(EXEC, 0, &["a.dll"]);
        let header = header::parse(&bytes).unwrap();
        let load = read(&bytes, &header);
        assert_eq!(load.kind, Kind::Dynamic);
        assert!(!load.pie);
    }

    #[test]
    fn a_dll_stays_shared_when_the_pie_bit_is_set() {
        let bytes = image(EXEC | DLL, DYNAMIC_BASE, &["a.dll"]);
        let header = header::parse(&bytes).unwrap();
        let load = read(&bytes, &header);
        assert_eq!(load.kind, Kind::Shared);
        assert_eq!(load.needed, ["a.dll"]);
        assert!(load.pie);
    }

    #[test]
    fn stripped_relocations_clear_the_pie_bit() {
        let bytes = image(EXEC | RELOCS_STRIPPED, DYNAMIC_BASE, &[]);
        let header = header::parse(&bytes).unwrap();
        let load = read(&bytes, &header);
        assert!(!load.pie);
        assert_eq!(load.kind, Kind::Static);
    }

    #[test]
    fn coff_object_is_rel() {
        let mut bytes = vec![0u8; 20];
        bytes[0] = 0x64;
        bytes[1] = 0x86;
        let header = header::parse(&bytes).unwrap();
        let load = read(&bytes, &header);
        assert_eq!(load.kind, Kind::Rel);
    }

    #[test]
    fn delay_names_stay_out_of_needed_and_keep_file_order() {
        let bytes = delay_image(&["a.dll"], &["late.dll", "other.dll"], true);
        let header = header::parse(&bytes).unwrap();
        let load = read(&bytes, &header);
        assert_eq!(load.kind, Kind::DynamicPie);
        assert_eq!(load.needed, ["a.dll"]);
        assert_eq!(load.delay, ["late.dll", "other.dll"]);
    }

    #[test]
    fn os_load_allows_kernel32_and_rejects_iconv() {
        let bytes = image(EXEC, DYNAMIC_BASE, &["KERNEL32.dll"]);
        let file = crate::open(&bytes, false).unwrap();
        let set = crate::policy::parse_set(&[(
            "bin/x",
            "format = \"pe\"\nIMAGE_DLLCHARACTERISTICS_DYNAMIC_BASE = true\nIMAGE_IMPORT_DESCRIPTOR = [{ name = \"KERNEL32.dll\", why = \"The Windows base library.\" }]\n",
        )])
        .unwrap();
        assert!(crate::osload::judge("bin/x", &file, &set).is_ok());

        let bytes = image(EXEC, DYNAMIC_BASE, &["KERNEL32.dll", "libiconv.dll"]);
        let file = crate::open(&bytes, false).unwrap();
        let err = crate::osload::judge("bin/x", &file, &set).unwrap_err();
        assert!(err.contains("libiconv.dll"), "{err}");

        let bytes = delay_image(&["KERNEL32.dll"], &["libiconv.dll"], true);
        let file = crate::open(&bytes, false).unwrap();
        let err = crate::osload::judge("bin/x", &file, &set).unwrap_err();
        assert!(err.contains("libiconv.dll"), "{err}");
    }

    #[test]
    fn delay_alone_keeps_a_static_pie_kind() {
        let bytes = delay_image(&[], &["late.dll"], true);
        let header = header::parse(&bytes).unwrap();
        let load = read(&bytes, &header);
        assert_eq!(load.kind, Kind::StaticPie);
        assert!(load.needed.is_empty());
        assert_eq!(load.delay, ["late.dll"]);
    }

    #[test]
    fn old_delay_descriptor_uses_a_virtual_address() {
        let bytes = delay_image(&[], &["late.dll"], false);
        let header = header::parse(&bytes).unwrap();
        assert_eq!(header.image_base, 0x40_0000);
        let load = read(&bytes, &header);
        assert_eq!(load.delay, ["late.dll"]);
    }

    #[test]
    fn hostile_delay_imports_do_not_panic() {
        let mut bytes = delay_image(&[], &["late.dll"], true);
        let header = header::parse(&bytes).unwrap();
        let opt = header.opt_off as usize;
        put_u32(&mut bytes, opt + 216, u32::MAX);
        put_u32(&mut bytes, opt + 220, u32::MAX);
        assert!(std::panic::catch_unwind(|| read(&bytes, &header)).is_ok());
        put_u32(&mut bytes, opt + 216, 0x1080);
        put_u32(&mut bytes, opt + 220, 4);
        assert!(std::panic::catch_unwind(|| read(&bytes, &header)).is_ok());
        put_u32(&mut bytes, opt + 108, 2);
        assert!(read(&bytes, &header).delay.is_empty());
    }

    #[test]
    fn hostile_imports_do_not_panic() {
        let mut bytes = image(EXEC, DYNAMIC_BASE, &["a.dll"]);
        let header = header::parse(&bytes).unwrap();
        bytes[header.section_off as usize + 12..header.section_off as usize + 16]
            .copy_from_slice(&u32::MAX.to_le_bytes());
        assert!(std::panic::catch_unwind(|| read(&bytes, &header)).is_ok());
        let mut huge = image(EXEC, 0, &[]);
        huge[0x46..0x48].copy_from_slice(&u16::MAX.to_le_bytes());
        let header = header::parse(&huge).unwrap();
        assert!(std::panic::catch_unwind(|| read(&huge, &header)).is_ok());
    }
}
