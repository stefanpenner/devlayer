/*
---
why: This file reads how an ELF file starts.
uses: bininfo
how: bininfo calls this file to read program headers.
This file does not print the page.
---
*/

//! How an ELF file starts, and which files it must open.
//! This module reads program headers. It does not print them.

use super::header::{Class, Data, Header, Type};
pub(crate) use crate::kind::name as kind_name;
pub use crate::kind::Kind;

const PT_LOAD: u32 = 1;
const PT_DYNAMIC: u32 = 2;
const PT_INTERP: u32 = 3;
const DT_NULL: u64 = 0;
const DT_NEEDED: u64 = 1;
const DT_STRTAB: u64 = 5;
const DT_RPATH: u64 = 15;
const DT_RUNPATH: u64 = 29;
const DT_FLAGS_1: u64 = 0x6fff_fffb;
const DF_1_PIE: u64 = 0x0800_0000;
const NAME_CAP: usize = 4096;
const IMPORT_CAP: usize = 256;

/// Facts from the program headers and from `PT_DYNAMIC`.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Load {
    pub kind: Kind,             // static, dynamic-pie, static-pie, or shared.
    pub interp: Option<String>, // Path of the loader. None means this file starts itself.
    pub needed: Vec<String>,    // Library names the loader opens.
    pub runpath: Vec<String>,   // DT_RUNPATH directories. A colon splits one string.
    pub rpath: Vec<String>,     // DT_RPATH directories. A colon splits one string.
    pub delay: Vec<String>,     // Always empty. ELF has no delay-import record.
    pub pie: bool,              // PIE bit in DT_FLAGS_1.
}

pub fn read(bytes: &[u8], header: &Header) -> Load {
    let rows = program_headers(bytes, header);
    let interp = rows.iter().find_map(|row| interp_path(bytes, row));
    let (pie, needed, runpath, rpath) = rows
        .iter()
        .find(|row| row.ty == PT_DYNAMIC)
        .map(|row| dynamic_facts(bytes, header, &rows, row))
        .unwrap_or((false, Vec::new(), Vec::new(), Vec::new()));
    Load {
        kind: classify(header.kind, interp.is_some(), pie),
        interp,
        needed,
        runpath,
        rpath,
        delay: Vec::new(),
        pie,
    }
}

pub(crate) fn kind_note(kind: Kind) -> &'static str {
    match kind {
        Kind::Static => "EXEC and no loader.",
        Kind::Dynamic => "A loader is present and the PIE bit is clear.",
        Kind::DynamicPie => "A loader is present and the PIE bit is set.",
        Kind::StaticPie => "DYN, no loader, and the PIE bit is set.",
        Kind::Shared => "DYN, no loader, and the PIE bit is clear.",
        Kind::Rel => "REL. The kernel does not run this file.",
        Kind::Other => "This e_type is outside the four start kinds.",
    }
}

fn classify(ty: Type, interp: bool, pie: bool) -> Kind {
    match ty {
        Type::Rel => Kind::Rel,
        Type::Exec if !interp => Kind::Static,
        Type::Exec | Type::Dyn if interp && pie => Kind::DynamicPie,
        Type::Exec | Type::Dyn if interp => Kind::Dynamic,
        Type::Dyn if pie => Kind::StaticPie,
        Type::Dyn => Kind::Shared,
        _ => Kind::Other,
    }
}

pub(crate) struct Phdr {
    pub(crate) ty: u32,
    pub(crate) offset: u64,
    pub(crate) vaddr: u64,
    pub(crate) filesz: u64,
    pub(crate) align: u64,
}

pub(crate) fn program_headers(bytes: &[u8], header: &Header) -> Vec<Phdr> {
    let need = match header.class {
        Class::Elf32 => 32,
        Class::Elf64 => 56,
    };
    if header.phnum == 0 || (header.phentsize as usize) < need {
        return Vec::new();
    }
    let mut rows = Vec::new();
    for index in 0..header.phnum {
        let Some(at) = row_start(header, index) else {
            break;
        };
        if at.checked_add(need).is_none_or(|end| end > bytes.len()) {
            break;
        }
        rows.push(decode_phdr(bytes, at, header));
    }
    rows
}

fn row_start(header: &Header, index: u16) -> Option<usize> {
    let step = header.phentsize as u64;
    let at = header.phoff.checked_add(step.checked_mul(index as u64)?)?;
    usize::try_from(at).ok()
}

fn decode_phdr(bytes: &[u8], at: usize, header: &Header) -> Phdr {
    match header.class {
        Class::Elf64 => Phdr {
            ty: u32_at(bytes, at, header.data),
            offset: u64_at(bytes, at + 8, header.data),
            vaddr: u64_at(bytes, at + 16, header.data),
            filesz: u64_at(bytes, at + 32, header.data),
            align: u64_at(bytes, at + 48, header.data),
        },
        Class::Elf32 => Phdr {
            ty: u32_at(bytes, at, header.data),
            offset: u32_at(bytes, at + 4, header.data) as u64,
            vaddr: u32_at(bytes, at + 8, header.data) as u64,
            filesz: u32_at(bytes, at + 16, header.data) as u64,
            align: u32_at(bytes, at + 28, header.data) as u64,
        },
    }
}

fn interp_path(bytes: &[u8], row: &Phdr) -> Option<String> {
    if row.ty != PT_INTERP {
        return None;
    }
    Some(read_text(bytes, row.offset, row.filesz))
}

fn dynamic_facts(
    bytes: &[u8],
    header: &Header,
    rows: &[Phdr],
    row: &Phdr,
) -> (bool, Vec<String>, Vec<String>, Vec<String>) {
    let mut strtab = None;
    let mut flags1 = 0u64;
    let mut needed_at = Vec::new();
    let mut runpath_at = Vec::new();
    let mut rpath_at = Vec::new();
    for (tag, val) in dyn_pairs(bytes, header, row) {
        match tag {
            DT_NULL => break,
            DT_STRTAB => strtab = Some(val),
            DT_FLAGS_1 => flags1 = val,
            DT_NEEDED => needed_at.push(val),
            DT_RUNPATH => runpath_at.push(val),
            DT_RPATH => rpath_at.push(val),
            _ => {}
        }
    }
    let pie = flags1 & DF_1_PIE != 0;
    let Some(base) = strtab.and_then(|vaddr| vaddr_to_offset(rows, vaddr)) else {
        return (pie, Vec::new(), Vec::new(), Vec::new());
    };
    (
        pie,
        names_at(bytes, base, &needed_at),
        split_dirs(names_at(bytes, base, &runpath_at)),
        split_dirs(names_at(bytes, base, &rpath_at)),
    )
}

fn names_at(bytes: &[u8], base: u64, offs: &[u64]) -> Vec<String> {
    offs.iter()
        .take(IMPORT_CAP)
        .filter_map(|off| base.checked_add(*off))
        .map(|at| read_text(bytes, at, NAME_CAP as u64))
        .filter(|name| !name.is_empty())
        .collect()
}

fn split_dirs(raws: Vec<String>) -> Vec<String> {
    let mut out = Vec::new();
    for raw in raws {
        for part in raw.split(':') {
            if part.is_empty() {
                continue;
            }
            if out.len() == IMPORT_CAP {
                return out;
            }
            out.push(part.to_string());
        }
    }
    out
}

pub(crate) fn dyn_pairs(bytes: &[u8], header: &Header, row: &Phdr) -> Vec<(u64, u64)> {
    let step = match header.class {
        Class::Elf32 => 8,
        Class::Elf64 => 16,
    };
    let Some(start) = usize::try_from(row.offset).ok() else {
        return Vec::new();
    };
    if start >= bytes.len() {
        return Vec::new();
    }
    let avail = bytes.len() - start;
    let span = usize::try_from(row.filesz).unwrap_or(usize::MAX).min(avail);
    let mut pairs = Vec::new();
    let mut at = start;
    while at + step <= start + span {
        let (tag, val) = match header.class {
            Class::Elf64 => (
                u64_at(bytes, at, header.data),
                u64_at(bytes, at + 8, header.data),
            ),
            Class::Elf32 => (
                u32_at(bytes, at, header.data) as u64,
                u32_at(bytes, at + 4, header.data) as u64,
            ),
        };
        pairs.push((tag, val));
        if tag == DT_NULL {
            break;
        }
        at += step;
    }
    pairs
}

pub(crate) fn vaddr_to_offset(rows: &[Phdr], vaddr: u64) -> Option<u64> {
    rows.iter().find_map(|row| {
        if row.ty != PT_LOAD || vaddr < row.vaddr {
            return None;
        }
        let delta = vaddr - row.vaddr;
        if delta < row.filesz {
            row.offset.checked_add(delta)
        } else {
            None
        }
    })
}

pub(crate) fn read_text(bytes: &[u8], offset: u64, limit: u64) -> String {
    let Ok(start) = usize::try_from(offset) else {
        return String::new();
    };
    if start >= bytes.len() {
        return String::new();
    }
    let room = (bytes.len() - start) as u64;
    let take = usize::try_from(limit.min(room).min(NAME_CAP as u64)).unwrap_or(0);
    let slice = &bytes[start..start + take];
    let end = slice
        .iter()
        .position(|byte| *byte == 0)
        .unwrap_or(slice.len());
    String::from_utf8_lossy(&slice[..end]).into_owned()
}

fn u32_at(bytes: &[u8], at: usize, data: Data) -> u32 {
    let b = [bytes[at], bytes[at + 1], bytes[at + 2], bytes[at + 3]];
    match data {
        Data::Little => u32::from_le_bytes(b),
        Data::Big => u32::from_be_bytes(b),
    }
}

fn u64_at(bytes: &[u8], at: usize, data: Data) -> u64 {
    let b = [
        bytes[at],
        bytes[at + 1],
        bytes[at + 2],
        bytes[at + 3],
        bytes[at + 4],
        bytes[at + 5],
        bytes[at + 6],
        bytes[at + 7],
    ];
    match data {
        Data::Little => u64::from_le_bytes(b),
        Data::Big => u64::from_be_bytes(b),
    }
}

#[cfg(test)]
mod tests {
    use super::super::header::{self, Data, Type};
    use super::*;

    fn classify_of(ty: Type, interp: bool, pie: bool) -> Kind {
        classify(ty, interp, pie)
    }

    #[test]
    fn the_four_start_kinds() {
        assert_eq!(classify_of(Type::Exec, false, false), Kind::Static);
        assert_eq!(classify_of(Type::Dyn, true, true), Kind::DynamicPie);
        assert_eq!(classify_of(Type::Dyn, false, true), Kind::StaticPie);
        assert_eq!(classify_of(Type::Dyn, false, false), Kind::Shared);
        assert_eq!(classify_of(Type::Exec, true, false), Kind::Dynamic);
        assert_eq!(classify_of(Type::Rel, false, false), Kind::Rel);
    }

    #[test]
    fn static_exec_names_no_library() {
        let bytes = elf_header(Type::Exec, 0, 0);
        let header = header::parse(&bytes).unwrap();
        let load = read(&bytes, &header);
        assert_eq!(load.kind, Kind::Static);
        assert_eq!(load.interp, None);
        assert!(load.needed.is_empty());
        assert!(load.runpath.is_empty());
        assert!(load.rpath.is_empty());
        assert!(!load.pie);
    }

    #[test]
    fn static_pie_sets_the_pie_bit_and_names_no_loader() {
        let bytes = elf_with_dynamic(Type::Dyn, DF_1_PIE, None, &[]);
        let header = header::parse(&bytes).unwrap();
        let load = read(&bytes, &header);
        assert_eq!(load.kind, Kind::StaticPie);
        assert_eq!(load.interp, None);
        assert!(load.needed.is_empty());
        assert!(load.pie);
    }

    #[test]
    fn dynamic_pie_names_the_loader_and_one_library() {
        let bytes = elf_with_dynamic(
            Type::Dyn,
            DF_1_PIE,
            Some("/lib/ld.so"),
            &["libc.so.6", "libm.so.6"],
        );
        let header = header::parse(&bytes).unwrap();
        let load = read(&bytes, &header);
        assert_eq!(load.kind, Kind::DynamicPie);
        assert_eq!(load.interp.as_deref(), Some("/lib/ld.so"));
        assert_eq!(
            load.needed,
            vec!["libc.so.6".to_string(), "libm.so.6".to_string()]
        );
        assert!(load.pie);
    }

    #[test]
    fn search_paths_keep_tag_order_and_split_on_colon() {
        let bytes = elf_with_dynamic_endian(
            Data::Little,
            Type::Dyn,
            0,
            Some("/lib/ld.so"),
            &["libc.so.6"],
            Some("$ORIGIN/../lib:/opt/lib"),
            Some("/old"),
        );
        let header = header::parse(&bytes).unwrap();
        let load = read(&bytes, &header);
        assert_eq!(load.needed, ["libc.so.6"]);
        assert_eq!(load.runpath, ["$ORIGIN/../lib", "/opt/lib"]);
        assert_eq!(load.rpath, ["/old"]);
    }

    #[test]
    fn shared_library_has_no_pie_bit() {
        let bytes = elf_with_dynamic(Type::Dyn, 0, None, &[]);
        let header = header::parse(&bytes).unwrap();
        let load = read(&bytes, &header);
        assert_eq!(load.kind, Kind::Shared);
        assert!(!load.pie);
    }

    #[test]
    fn os_load_allows_libc_and_rejects_iconv() {
        let good = elf_with_dynamic(
            Type::Dyn,
            0,
            Some("/lib/ld-linux-aarch64.so.1"),
            &["libc.so.6"],
        );
        let file = crate::open(&good, false).unwrap();
        let policy = crate::policy::devlayer().unwrap();
        assert!(crate::osload::host_libraries(&file, &policy)
            .unwrap()
            .is_empty());

        let bad = elf_with_dynamic(
            Type::Dyn,
            0,
            Some("/opt/homebrew/lib/libz.so"),
            &["libiconv.so.2"],
        );
        let file = crate::open(&bad, false).unwrap();
        assert_eq!(
            crate::osload::host_libraries(&file, &policy).unwrap(),
            vec![
                "/opt/homebrew/lib/libz.so".to_string(),
                "libiconv.so.2".to_string(),
            ]
        );
    }

    #[test]
    fn big_endian_dynamic_keeps_the_pie_bit() {
        let bytes = elf_with_dynamic_endian(Data::Big, Type::Dyn, DF_1_PIE, None, &[], None, None);
        let header = header::parse(&bytes).unwrap();
        let load = read(&bytes, &header);
        assert_eq!(header.data, Data::Big);
        assert_eq!(load.kind, Kind::StaticPie);
        assert!(load.pie);
    }

    #[test]
    fn needed_list_is_capped() {
        let names: Vec<String> = (0..300).map(|index| format!("l{index}.so")).collect();
        let refs: Vec<&str> = names.iter().map(String::as_str).collect();
        let bytes = elf_with_dynamic(Type::Dyn, 0, None, &refs);
        let header = header::parse(&bytes).unwrap();
        let load = read(&bytes, &header);
        assert!(load.needed.len() <= 256, "{}", load.needed.len());
        assert_eq!(load.needed.len(), 256);
    }

    #[test]
    fn hostile_program_headers_do_not_panic() {
        let mut samples = vec![
            vec![],
            vec![0u8; 64],
            elf_header(Type::Dyn, u64::MAX - 8, 65535),
        ];
        let mut huge = elf_header(Type::Exec, 64, 1);
        huge[54..56].copy_from_slice(&56u16.to_le_bytes());
        huge.truncate(70);
        samples.push(huge);
        for bytes in samples {
            let parsed = std::panic::catch_unwind(|| {
                if let Ok(header) = header::parse(&bytes) {
                    read(&bytes, &header);
                }
            });
            assert!(parsed.is_ok(), "load read panicked");
        }
    }

    fn elf_header(ty: Type, phoff: u64, phnum: u16) -> Vec<u8> {
        let mut bytes = vec![0u8; 64];
        bytes[0..4].copy_from_slice(&[0x7f, b'E', b'L', b'F']);
        bytes[4] = 2;
        bytes[5] = 1;
        bytes[6] = 1;
        put_u16(&mut bytes, 16, type_byte(ty), Data::Little);
        put_u16(&mut bytes, 18, 62, Data::Little);
        put_u32(&mut bytes, 20, 1, Data::Little);
        put_u64(&mut bytes, 32, phoff, Data::Little);
        put_u16(&mut bytes, 52, 64, Data::Little);
        put_u16(&mut bytes, 54, 56, Data::Little);
        put_u16(&mut bytes, 56, phnum, Data::Little);
        bytes
    }

    fn elf_with_dynamic(ty: Type, flags1: u64, interp: Option<&str>, needed: &[&str]) -> Vec<u8> {
        elf_with_dynamic_endian(Data::Little, ty, flags1, interp, needed, None, None)
    }

    fn elf_with_dynamic_endian(
        data: Data,
        ty: Type,
        flags1: u64,
        interp: Option<&str>,
        needed: &[&str],
        runpath: Option<&str>,
        rpath: Option<&str>,
    ) -> Vec<u8> {
        let mut names = Vec::new();
        let mut needed_at = Vec::new();
        for name in needed {
            needed_at.push(names.len() as u64);
            names.extend(name.as_bytes());
            names.push(0);
        }
        let run_at = runpath.map(|text| push_name(&mut names, text));
        let r_at = rpath.map(|text| push_name(&mut names, text));
        if names.is_empty() {
            names.push(0);
        }
        let interp_bytes = interp.map(|path| {
            let mut bytes = path.as_bytes().to_vec();
            bytes.push(0);
            bytes
        });
        let phnum: u16 = if interp_bytes.is_some() { 3 } else { 2 };
        let ph_bytes = phnum as usize * 56;
        let extra = run_at.is_some() as usize + r_at.is_some() as usize;
        let dyn_entries = 3 + needed.len() + extra;
        let dyn_bytes = dyn_entries * 16;
        let header_len = 64;
        let dyn_off = header_len + ph_bytes;
        let str_off = dyn_off + dyn_bytes;
        let interp_off = str_off + names.len();
        let mut bytes = vec![0u8; interp_off + interp_bytes.as_ref().map(|b| b.len()).unwrap_or(0)];
        bytes[0..4].copy_from_slice(&[0x7f, b'E', b'L', b'F']);
        bytes[4] = 2;
        bytes[5] = if data == Data::Little { 1 } else { 2 };
        bytes[6] = 1;
        put_u16(&mut bytes, 16, type_byte(ty), data);
        put_u16(&mut bytes, 18, 62, data);
        put_u32(&mut bytes, 20, 1, data);
        put_u64(&mut bytes, 32, header_len as u64, data);
        put_u16(&mut bytes, 52, 64, data);
        put_u16(&mut bytes, 54, 56, data);
        put_u16(&mut bytes, 56, phnum, data);
        let filesz = bytes.len() as u64;
        write_load(&mut bytes, header_len, 0, 0, filesz, data);
        write_dynamic(
            &mut bytes,
            header_len + 56,
            dyn_off as u64,
            dyn_bytes as u64,
            data,
        );
        if interp_bytes.is_some() {
            write_interp(
                &mut bytes,
                header_len + 112,
                interp_off as u64,
                interp_bytes.as_ref().unwrap().len() as u64,
                data,
            );
        }
        write_dyn_table(
            &mut bytes,
            dyn_off,
            str_off as u64,
            flags1,
            &needed_at,
            run_at,
            r_at,
            data,
        );
        bytes[str_off..str_off + names.len()].copy_from_slice(&names);
        if let Some(text) = interp_bytes {
            bytes[interp_off..interp_off + text.len()].copy_from_slice(&text);
        }
        bytes
    }

    fn write_load(bytes: &mut [u8], at: usize, offset: u64, vaddr: u64, filesz: u64, data: Data) {
        put_u32(bytes, at, PT_LOAD, data);
        put_u64(bytes, at + 8, offset, data);
        put_u64(bytes, at + 16, vaddr, data);
        put_u64(bytes, at + 32, filesz, data);
    }

    fn write_dynamic(bytes: &mut [u8], at: usize, offset: u64, filesz: u64, data: Data) {
        put_u32(bytes, at, PT_DYNAMIC, data);
        put_u64(bytes, at + 8, offset, data);
        put_u64(bytes, at + 16, offset, data);
        put_u64(bytes, at + 32, filesz, data);
    }

    fn write_interp(bytes: &mut [u8], at: usize, offset: u64, filesz: u64, data: Data) {
        put_u32(bytes, at, PT_INTERP, data);
        put_u64(bytes, at + 8, offset, data);
        put_u64(bytes, at + 16, offset, data);
        put_u64(bytes, at + 32, filesz, data);
    }

    fn push_name(names: &mut Vec<u8>, text: &str) -> u64 {
        let off = names.len() as u64;
        names.extend(text.as_bytes());
        names.push(0);
        off
    }

    fn write_dyn_table(
        bytes: &mut [u8],
        at: usize,
        strtab: u64,
        flags1: u64,
        needed_at: &[u64],
        run_at: Option<u64>,
        r_at: Option<u64>,
        data: Data,
    ) {
        put_u64(bytes, at, DT_STRTAB, data);
        put_u64(bytes, at + 8, strtab, data);
        put_u64(bytes, at + 16, DT_FLAGS_1, data);
        put_u64(bytes, at + 24, flags1, data);
        let mut slot = at + 32;
        for name_at in needed_at {
            put_u64(bytes, slot, DT_NEEDED, data);
            put_u64(bytes, slot + 8, *name_at, data);
            slot += 16;
        }
        if let Some(name_at) = run_at {
            put_u64(bytes, slot, DT_RUNPATH, data);
            put_u64(bytes, slot + 8, name_at, data);
            slot += 16;
        }
        if let Some(name_at) = r_at {
            put_u64(bytes, slot, DT_RPATH, data);
            put_u64(bytes, slot + 8, name_at, data);
            slot += 16;
        }
        put_u64(bytes, slot, DT_NULL, data);
    }

    fn type_byte(ty: Type) -> u16 {
        match ty {
            Type::None => 0,
            Type::Rel => 1,
            Type::Exec => 2,
            Type::Dyn => 3,
            Type::Core => 4,
            Type::Other(n) => n,
        }
    }

    fn put_u16(buf: &mut [u8], at: usize, v: u16, data: Data) {
        let b = match data {
            Data::Little => v.to_le_bytes(),
            Data::Big => v.to_be_bytes(),
        };
        buf[at..at + 2].copy_from_slice(&b);
    }

    fn put_u32(buf: &mut [u8], at: usize, v: u32, data: Data) {
        let b = match data {
            Data::Little => v.to_le_bytes(),
            Data::Big => v.to_be_bytes(),
        };
        buf[at..at + 4].copy_from_slice(&b);
    }

    fn put_u64(buf: &mut [u8], at: usize, v: u64, data: Data) {
        let b = match data {
            Data::Little => v.to_le_bytes(),
            Data::Big => v.to_be_bytes(),
        };
        buf[at..at + 8].copy_from_slice(&b);
    }
}
