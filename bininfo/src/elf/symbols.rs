/*
---
why: This file reads ELF dynamic symbols and GNU notes.
uses: bininfo
how: bininfo calls this file to count symbols and name the entry.
This file does not print the page.
---
*/

//! Dynamic symbols, version needs, and GNU notes.
//! Counts come from PT_DYNAMIC. Section headers only mark .symtab and notes.
//! This module does not print.

use super::header::{Class, Data, Header};
use super::load::{self, Phdr};
use crate::claim::{CpuFloor, Version};

const PT_DYNAMIC: u32 = 2;
const PT_NOTE: u32 = 4;
const DT_STRTAB: u64 = 5;
const DT_SYMTAB: u64 = 6;
const DT_SYMENT: u64 = 11;
const DT_HASH: u64 = 4;
const DT_GNU_HASH: u64 = 0x6fff_fef5;
const DT_VERSYM: u64 = 0x6fff_fff0;
const DT_VERNEED: u64 = 0x6fff_fffe;
const DT_VERNEEDNUM: u64 = 0x6fff_ffff;
const SHT_SYMTAB: u32 = 2;
const SHT_NOTE: u32 = 7;
const SYM_CAP: u32 = 16384;
const FILE_CAP: usize = 32;
const AUX_CAP: usize = 64;
const NOTE_CAP: usize = 64;
const NAME_CAP: u64 = 4096;
const NT_GNU_ABI_TAG: u32 = 1;
const NT_GNU_PROPERTY_TYPE_0: u32 = 5;
const GNU_PROPERTY_AARCH64_FEATURE_1_AND: u32 = 0xc000_0000;
const GNU_PROPERTY_X86_ISA_1_NEEDED: u32 = 0xc000_8002;

/// One undefined dynamic symbol.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Undef {
    pub name: String,
    pub version: Option<String>,
    pub binding: &'static str,
}

/// One version this file requires from one shared object.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct VerNeed {
    pub file: String,
    pub need: String,
}

/// NT_GNU_ABI_TAG. `os` 0 is Linux.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct AbiTag {
    pub os: u32,
    pub major: u32,
    pub minor: u32,
    pub patch: u32,
}

impl AbiTag {
    pub fn text(self) -> String {
        let name = match self.os {
            0 => "linux",
            1 => "hurd",
            2 => "solaris",
            3 => "freebsd",
            4 => "netbsd",
            os => return format!("{os} {}.{}.{}", self.major, self.minor, self.patch),
        };
        format!("{name} {}.{}.{}", self.major, self.minor, self.patch)
    }
}

/// Loader symbol facts. Row lists stay empty unless the caller asked for them.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Symbols {
    pub undef: u32,
    pub defined: u32,
    pub undef_rows: Vec<Undef>,
    pub verneed: Vec<VerNeed>,
    pub symtab: bool,
    pub abi_tag: Option<AbiTag>,
    pub property: Vec<String>,
    pub entry_name: Option<String>,
    pub entry_note: &'static str,
    pub rows: bool,
}

impl Symbols {
    #[cfg(test)]
    pub fn empty() -> Self {
        Self {
            undef: 0,
            defined: 0,
            undef_rows: Vec::new(),
            verneed: Vec::new(),
            symtab: false,
            abi_tag: None,
            property: Vec::new(),
            entry_name: None,
            entry_note: "[SHT_DYNSYM] No symbol value equals e_entry.",
            rows: false,
        }
    }

    pub fn has_abi(&self) -> bool {
        self.abi_tag.is_some() || !self.property.is_empty()
    }
}

struct Tags {
    strtab: Option<u64>,
    symtab: Option<u64>,
    syment: u64,
    hash: Option<u64>,
    gnu: Option<u64>,
    versym: Option<u64>,
    verneed: Option<u64>,
    verneednum: u64,
}

struct Sec {
    ty: u32,
    offset: u64,
    size: u64,
    link: u32,
    align: u64,
}

pub fn read(bytes: &[u8], header: &Header, rows: bool) -> Symbols {
    let phdrs = load::program_headers(bytes, header);
    let tags = tags_of(bytes, header, &phdrs);
    let str_base = tags
        .strtab
        .and_then(|vaddr| load::vaddr_to_offset(&phdrs, vaddr));
    let count = symbol_count(bytes, header, &phdrs, &tags);
    let versions = version_index(bytes, header, &phdrs, &tags);
    let (undef, defined, undef_rows) = tally(
        bytes, header, &phdrs, &tags, str_base, count, &versions, rows,
    );
    let verneed = verneed_rows(bytes, header, &phdrs, str_base, &tags);
    let sections = sections_of(bytes, header);
    let symtab = sections.iter().any(|sec| sec.ty == SHT_SYMTAB);
    let (entry_name, entry_note) =
        entry_of(bytes, header, &phdrs, &tags, str_base, count, &sections);
    let (abi_tag, property) = notes_of(bytes, header, &phdrs, &sections);
    Symbols {
        undef,
        defined,
        undef_rows,
        verneed,
        symtab,
        abi_tag,
        property,
        entry_name,
        entry_note,
        rows,
    }
}

/// One floor. The ABI tag is a Linux kernel version when `os` is 0.
pub fn floors(header: &Header, symbols: &Symbols) -> Vec<CpuFloor> {
    let word = super::words::machine_name(header.machine).unwrap_or("");
    let linux = symbols.abi_tag.filter(|tag| tag.os == 0);
    let minos = linux.map(|tag| Version {
        major: tag.major,
        minor: tag.minor,
        patch: tag.patch,
    });
    let os_family = if linux.is_some() { "linux" } else { "" };
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

fn tags_of(bytes: &[u8], header: &Header, phdrs: &[Phdr]) -> Tags {
    let mut tags = Tags {
        strtab: None,
        symtab: None,
        syment: sym_step(header) as u64,
        hash: None,
        gnu: None,
        versym: None,
        verneed: None,
        verneednum: 0,
    };
    let Some(row) = phdrs.iter().find(|row| row.ty == PT_DYNAMIC) else {
        return tags;
    };
    for (tag, val) in load::dyn_pairs(bytes, header, row) {
        match tag {
            DT_STRTAB => tags.strtab = Some(val),
            DT_SYMTAB => tags.symtab = Some(val),
            DT_SYMENT if val == 16 || val == 24 => tags.syment = val,
            DT_HASH => tags.hash = Some(val),
            DT_GNU_HASH => tags.gnu = Some(val),
            DT_VERSYM => tags.versym = Some(val),
            DT_VERNEED => tags.verneed = Some(val),
            DT_VERNEEDNUM => tags.verneednum = val,
            _ => {}
        }
    }
    tags
}

fn symbol_count(bytes: &[u8], header: &Header, phdrs: &[Phdr], tags: &Tags) -> u32 {
    let Some(sym_at) = file_at(bytes, phdrs, tags.symtab) else {
        return 0;
    };
    if let Some(hash) = tags.hash {
        let str_at = file_at(bytes, phdrs, tags.strtab);
        if let Some(n) = sysv_nchain(bytes, header.data, phdrs, hash, sym_at, str_at, tags.syment) {
            return n;
        }
    }
    if let Some(gnu) = tags.gnu {
        if let Some(n) = gnu_count(bytes, header, phdrs, gnu) {
            return n;
        }
    }
    0
}

fn sysv_nchain(
    bytes: &[u8],
    data: Data,
    phdrs: &[Phdr],
    hash: u64,
    sym_at: usize,
    str_at: Option<usize>,
    syment: u64,
) -> Option<u32> {
    let at = file_at(bytes, phdrs, Some(hash))?;
    let nchain = u32_at(bytes, at + 4, data)?;
    let step = syment.max(1) as usize;
    let mut end = bytes.len();
    if let Some(str_at) = str_at {
        if str_at > sym_at {
            end = end.min(str_at);
        }
    }
    let fit = end.saturating_sub(sym_at) / step;
    Some(nchain.min(fit as u32).min(SYM_CAP))
}

fn gnu_count(bytes: &[u8], header: &Header, phdrs: &[Phdr], gnu: u64) -> Option<u32> {
    let at = file_at(bytes, phdrs, Some(gnu))?;
    let nbuckets = u32_at(bytes, at, header.data)?;
    let symoff = u32_at(bytes, at + 4, header.data)?;
    let bloom_size = u32_at(bytes, at + 8, header.data)?;
    let bloom_ent = match header.class {
        Class::Elf64 => 8usize,
        Class::Elf32 => 4,
    };
    let bloom_bytes = (bloom_size as usize).checked_mul(bloom_ent)?;
    let buckets = at.checked_add(16)?.checked_add(bloom_bytes)?;
    let nb = (nbuckets as usize).min(SYM_CAP as usize);
    let chains = buckets.checked_add(nb.saturating_mul(4))?;
    if chains > bytes.len() || symoff > SYM_CAP {
        return None;
    }
    let mut max = symoff;
    for index in 0..nb {
        let bucket = u32_at(bytes, buckets + index * 4, header.data)?;
        if bucket < symoff {
            continue;
        }
        let mut idx = bucket;
        for _ in 0..64 {
            let chain_i = idx.checked_sub(symoff)? as usize;
            if chain_i >= SYM_CAP as usize {
                break;
            }
            let pos = chains.checked_add(chain_i * 4)?;
            let val = u32_at(bytes, pos, header.data)?;
            max = max.max(idx.saturating_add(1));
            if val & 1 != 0 {
                break;
            }
            idx = idx.saturating_add(1);
        }
    }
    Some(max.min(SYM_CAP))
}

fn tally(
    bytes: &[u8],
    header: &Header,
    phdrs: &[Phdr],
    tags: &Tags,
    str_base: Option<u64>,
    count: u32,
    versions: &[(u16, String)],
    rows: bool,
) -> (u32, u32, Vec<Undef>) {
    let Some(sym_at) = file_at(bytes, phdrs, tags.symtab) else {
        return (0, 0, Vec::new());
    };
    let step = tags.syment as usize;
    let mut undef = 0u32;
    let mut defined = 0u32;
    let mut undef_rows = Vec::new();
    for index in 1..count {
        let Some(at) = sym_at.checked_add(index as usize * step) else {
            break;
        };
        let Some((name_off, _value, info, shndx)) = decode_sym(bytes, at, header, step) else {
            break;
        };
        if shndx == 0 {
            undef = undef.saturating_add(1);
            if rows {
                if let Some(name) = symbol_name(bytes, str_base, name_off) {
                    undef_rows.push(Undef {
                        name,
                        version: version_of(versions, versym_at(bytes, header, phdrs, tags, index)),
                        binding: binding(info),
                    });
                }
            }
        } else if info >> 4 != 0 {
            defined = defined.saturating_add(1);
        }
    }
    (undef, defined, undef_rows)
}

fn version_index(bytes: &[u8], header: &Header, phdrs: &[Phdr], tags: &Tags) -> Vec<(u16, String)> {
    let mut out = Vec::new();
    let Some(base) = file_at(bytes, phdrs, tags.verneed) else {
        return out;
    };
    let Some(str_base) = file_off(phdrs, tags.strtab) else {
        return out;
    };
    let mut at = base;
    let mut seen = Vec::new();
    let mut files = 0usize;
    while files < FILE_CAP.min(tags.verneednum.max(1) as usize) {
        if !seen.iter().all(|old| *old != at) {
            break;
        }
        seen.push(at);
        let Some(vn_cnt) = u16_at(bytes, at + 2, header.data) else {
            break;
        };
        let Some(vn_aux) = u32_at(bytes, at + 8, header.data) else {
            break;
        };
        let Some(vn_next) = u32_at(bytes, at + 12, header.data) else {
            break;
        };
        let mut aux = at.saturating_add(vn_aux as usize);
        for _ in 0..vn_cnt.min(AUX_CAP as u16) {
            let Some(other) = u16_at(bytes, aux + 6, header.data) else {
                break;
            };
            let Some(name_off) = u32_at(bytes, aux + 8, header.data) else {
                break;
            };
            let Some(next) = u32_at(bytes, aux + 12, header.data) else {
                break;
            };
            if let Some(name) = symbol_name(bytes, Some(str_base), name_off) {
                if other > 1 && out.iter().all(|(id, _)| *id != other) {
                    out.push((other, name));
                }
            }
            if next == 0 {
                break;
            }
            let Some(next_at) = aux.checked_add(next as usize) else {
                break;
            };
            if next_at == aux {
                break;
            }
            aux = next_at;
        }
        files += 1;
        if vn_next == 0 {
            break;
        }
        let Some(next_at) = at.checked_add(vn_next as usize) else {
            break;
        };
        if next_at == at {
            break;
        }
        at = next_at;
    }
    out
}

fn verneed_rows(
    bytes: &[u8],
    header: &Header,
    phdrs: &[Phdr],
    str_base: Option<u64>,
    tags: &Tags,
) -> Vec<VerNeed> {
    let Some(base) = file_at(bytes, phdrs, tags.verneed) else {
        return Vec::new();
    };
    let Some(str_base) = str_base else {
        return Vec::new();
    };
    let mut out = Vec::new();
    let mut at = base;
    let mut seen = Vec::new();
    let limit = FILE_CAP.min(tags.verneednum.max(1) as usize);
    while out.len() < FILE_CAP && seen.len() < limit {
        if seen.contains(&at) {
            break;
        }
        seen.push(at);
        let Some(vn_cnt) = u16_at(bytes, at + 2, header.data) else {
            break;
        };
        let Some(file_off) = u32_at(bytes, at + 4, header.data) else {
            break;
        };
        let Some(vn_aux) = u32_at(bytes, at + 8, header.data) else {
            break;
        };
        let Some(vn_next) = u32_at(bytes, at + 12, header.data) else {
            break;
        };
        let Some(file) = symbol_name(bytes, Some(str_base), file_off) else {
            break;
        };
        let mut aux = match at.checked_add(vn_aux as usize) {
            Some(aux) => aux,
            None => break,
        };
        for _ in 0..(vn_cnt as usize).min(AUX_CAP) {
            let Some(name_off) = u32_at(bytes, aux + 8, header.data) else {
                break;
            };
            let Some(next) = u32_at(bytes, aux + 12, header.data) else {
                break;
            };
            if let Some(need) = symbol_name(bytes, Some(str_base), name_off) {
                out.push(VerNeed {
                    file: file.clone(),
                    need,
                });
            }
            if next == 0 {
                break;
            }
            let Some(next_at) = aux.checked_add(next as usize) else {
                break;
            };
            if next_at == aux {
                break;
            }
            aux = next_at;
        }
        if vn_next == 0 {
            break;
        }
        let Some(next_at) = at.checked_add(vn_next as usize) else {
            break;
        };
        if next_at == at {
            break;
        }
        at = next_at;
    }
    out
}

fn versym_at(
    bytes: &[u8],
    header: &Header,
    phdrs: &[Phdr],
    tags: &Tags,
    index: u32,
) -> Option<u16> {
    let at = file_at(bytes, phdrs, tags.versym)?;
    let pos = at.checked_add(index as usize * 2)?;
    u16_at(bytes, pos, header.data)
}

fn version_of(versions: &[(u16, String)], id: Option<u16>) -> Option<String> {
    let id = id.filter(|id| *id > 1)?;
    versions
        .iter()
        .find(|(other, _)| *other == id)
        .map(|(_, name)| name.clone())
}

fn entry_of(
    bytes: &[u8],
    header: &Header,
    phdrs: &[Phdr],
    tags: &Tags,
    str_base: Option<u64>,
    count: u32,
    sections: &[Sec],
) -> (Option<String>, &'static str) {
    if let Some(sym_at) = file_at(bytes, phdrs, tags.symtab) {
        if let Some(name) = find_value(
            bytes,
            header,
            sym_at,
            tags.syment as usize,
            count,
            str_base,
            header.entry,
        ) {
            return (
                Some(name),
                "[SHT_DYNSYM] Symbol whose value equals e_entry.",
            );
        }
    }
    if let Some(sec) = sections.iter().find(|sec| sec.ty == SHT_SYMTAB) {
        let step = sym_step(header);
        let count = sec_count(sec.size, step);
        let str_base = sections.get(sec.link as usize).map(|strtab| strtab.offset);
        if let Some(name) = find_value(
            bytes,
            header,
            sec.offset as usize,
            step,
            count,
            str_base,
            header.entry,
        ) {
            return (
                Some(name),
                "[SHT_SYMTAB] Symbol whose value equals e_entry.",
            );
        }
        return (None, "[SHT_SYMTAB] No symbol value equals e_entry.");
    }
    (None, "[SHT_DYNSYM] No symbol value equals e_entry.")
}

fn find_value(
    bytes: &[u8],
    header: &Header,
    sym_at: usize,
    step: usize,
    count: u32,
    str_base: Option<u64>,
    entry: u64,
) -> Option<String> {
    if step < 16 {
        return None;
    }
    for index in 0..count {
        let at = sym_at.checked_add(index as usize * step)?;
        let (name_off, value, _info, shndx) = decode_sym(bytes, at, header, step)?;
        if shndx == 0 || value != entry {
            continue;
        }
        if let Some(name) = symbol_name(bytes, str_base, name_off) {
            return Some(name);
        }
    }
    None
}

fn notes_of(
    bytes: &[u8],
    header: &Header,
    phdrs: &[Phdr],
    sections: &[Sec],
) -> (Option<AbiTag>, Vec<String>) {
    let mut tag = None;
    let mut property = Vec::new();
    let mut saw = false;
    for row in phdrs.iter().filter(|row| row.ty == PT_NOTE) {
        saw = true;
        parse_note_blob(
            bytes,
            header,
            row.offset,
            row.filesz,
            note_align(header, row.align),
            &mut tag,
            &mut property,
        );
    }
    if !saw {
        for sec in sections.iter().filter(|sec| sec.ty == SHT_NOTE) {
            parse_note_blob(
                bytes,
                header,
                sec.offset,
                sec.size,
                note_align(header, sec.align),
                &mut tag,
                &mut property,
            );
        }
    }
    (tag, property)
}

fn parse_note_blob(
    bytes: &[u8],
    header: &Header,
    offset: u64,
    size: u64,
    align: usize,
    tag: &mut Option<AbiTag>,
    property: &mut Vec<String>,
) {
    let Some(start) = usize::try_from(offset).ok() else {
        return;
    };
    if start >= bytes.len() {
        return;
    }
    let end = start
        .saturating_add(usize::try_from(size).unwrap_or(usize::MAX))
        .min(bytes.len());
    let mut at = start;
    for _ in 0..NOTE_CAP {
        if at + 12 > end {
            break;
        }
        let Some(namesz) = u32_at(bytes, at, header.data) else {
            break;
        };
        let Some(descsz) = u32_at(bytes, at + 4, header.data) else {
            break;
        };
        let Some(kind) = u32_at(bytes, at + 8, header.data) else {
            break;
        };
        let name_pad = align_up(namesz as usize, align);
        let desc_pad = align_up(descsz as usize, align);
        let Some(desc_off) = at.checked_add(12).and_then(|n| n.checked_add(name_pad)) else {
            break;
        };
        let Some(next) = desc_off.checked_add(desc_pad) else {
            break;
        };
        if next <= at || next > end {
            break;
        }
        let gnu = namesz >= 4 && bytes.get(at + 12..at + 16) == Some(b"GNU\0");
        if gnu && kind == NT_GNU_ABI_TAG && descsz >= 16 && tag.is_none() {
            if let Some(parsed) = abi_desc(bytes, desc_off, header.data) {
                *tag = Some(parsed);
            }
        }
        if gnu && kind == NT_GNU_PROPERTY_TYPE_0 {
            parse_properties(
                bytes,
                desc_off,
                descsz as usize,
                align,
                header.data,
                property,
            );
        }
        at = next;
    }
}

fn abi_desc(bytes: &[u8], at: usize, data: Data) -> Option<AbiTag> {
    Some(AbiTag {
        os: u32_at(bytes, at, data)?,
        major: u32_at(bytes, at + 4, data)?,
        minor: u32_at(bytes, at + 8, data)?,
        patch: u32_at(bytes, at + 12, data)?,
    })
}

fn parse_properties(
    bytes: &[u8],
    at: usize,
    size: usize,
    align: usize,
    data: Data,
    property: &mut Vec<String>,
) {
    let end = at.saturating_add(size).min(bytes.len());
    let mut cursor = at;
    for _ in 0..NOTE_CAP {
        if cursor + 8 > end {
            break;
        }
        let Some(kind) = u32_at(bytes, cursor, data) else {
            break;
        };
        let Some(datasz) = u32_at(bytes, cursor + 4, data) else {
            break;
        };
        let data_at = cursor + 8;
        if data_at.saturating_add(datasz as usize) > end {
            break;
        }
        if let Some(bits) = u32_at(bytes, data_at, data).filter(|_| datasz >= 4) {
            match kind {
                GNU_PROPERTY_X86_ISA_1_NEEDED => {
                    for (bit, name) in [
                        (1u32, "x86-64-baseline"),
                        (2, "x86-64-v2"),
                        (4, "x86-64-v3"),
                        (8, "x86-64-v4"),
                    ] {
                        if bits & bit != 0 {
                            property.push(name.to_string());
                        }
                    }
                }
                GNU_PROPERTY_AARCH64_FEATURE_1_AND => {
                    if bits & 1 != 0 {
                        property.push("bti".to_string());
                    }
                    if bits & 2 != 0 {
                        property.push("pac".to_string());
                    }
                }
                _ => {}
            }
        }
        let step = align_up(8 + datasz as usize, align);
        let Some(next) = cursor.checked_add(step.max(8)) else {
            break;
        };
        if next <= cursor || next > end {
            break;
        }
        cursor = next;
    }
}

fn sections_of(bytes: &[u8], header: &Header) -> Vec<Sec> {
    let need = match header.class {
        Class::Elf64 => 64usize,
        Class::Elf32 => 40,
    };
    if header.shnum == 0 || (header.shentsize as usize) < need {
        return Vec::new();
    }
    let mut out = Vec::new();
    for index in 0..header.shnum.min(4096) {
        let Some(at) = sh_at(header, index) else {
            break;
        };
        if at.checked_add(need).is_none_or(|end| end > bytes.len()) {
            break;
        }
        out.push(decode_sec(bytes, at, header));
    }
    out
}

fn sh_at(header: &Header, index: u16) -> Option<usize> {
    let step = header.shentsize as u64;
    let at = header.shoff.checked_add(step.checked_mul(index as u64)?)?;
    usize::try_from(at).ok()
}

fn decode_sec(bytes: &[u8], at: usize, header: &Header) -> Sec {
    match header.class {
        Class::Elf64 => Sec {
            ty: u32_at(bytes, at + 4, header.data).unwrap_or(0),
            offset: u64_at(bytes, at + 24, header.data).unwrap_or(0),
            size: u64_at(bytes, at + 32, header.data).unwrap_or(0),
            link: u32_at(bytes, at + 40, header.data).unwrap_or(0),
            align: u64_at(bytes, at + 48, header.data).unwrap_or(0),
        },
        Class::Elf32 => Sec {
            ty: u32_at(bytes, at + 4, header.data).unwrap_or(0),
            offset: u32_at(bytes, at + 16, header.data).unwrap_or(0) as u64,
            size: u32_at(bytes, at + 20, header.data).unwrap_or(0) as u64,
            link: u32_at(bytes, at + 24, header.data).unwrap_or(0),
            align: u32_at(bytes, at + 32, header.data).unwrap_or(0) as u64,
        },
    }
}

fn decode_sym(
    bytes: &[u8],
    at: usize,
    header: &Header,
    step: usize,
) -> Option<(u32, u64, u8, u16)> {
    if at.checked_add(step)? > bytes.len() {
        return None;
    }
    match header.class {
        Class::Elf64 => Some((
            u32_at(bytes, at, header.data)?,
            u64_at(bytes, at + 8, header.data)?,
            *bytes.get(at + 4)?,
            u16_at(bytes, at + 6, header.data)?,
        )),
        Class::Elf32 => Some((
            u32_at(bytes, at, header.data)?,
            u32_at(bytes, at + 4, header.data)? as u64,
            *bytes.get(at + 12)?,
            u16_at(bytes, at + 14, header.data)?,
        )),
    }
}

fn symbol_name(bytes: &[u8], str_base: Option<u64>, off: u32) -> Option<String> {
    let at = str_base?.checked_add(off as u64)?;
    let name = load::read_text(bytes, at, NAME_CAP);
    if name.is_empty() {
        None
    } else {
        Some(name)
    }
}

fn binding(info: u8) -> &'static str {
    match info >> 4 {
        0 => "local",
        1 => "global",
        2 => "weak",
        _ => "other",
    }
}

fn sym_step(header: &Header) -> usize {
    match header.class {
        Class::Elf64 => 24,
        Class::Elf32 => 16,
    }
}

fn sec_count(size: u64, step: usize) -> u32 {
    if step == 0 {
        return 0;
    }
    (size / step as u64).min(SYM_CAP as u64) as u32
}

fn file_at(bytes: &[u8], phdrs: &[Phdr], vaddr: Option<u64>) -> Option<usize> {
    let off = load::vaddr_to_offset(phdrs, vaddr?)?;
    let at = usize::try_from(off).ok()?;
    if at < bytes.len() {
        Some(at)
    } else {
        None
    }
}

fn file_off(phdrs: &[Phdr], vaddr: Option<u64>) -> Option<u64> {
    load::vaddr_to_offset(phdrs, vaddr?)
}

fn note_align(header: &Header, align: u64) -> usize {
    match align {
        4 | 8 => align as usize,
        _ => match header.class {
            Class::Elf64 => 8,
            Class::Elf32 => 4,
        },
    }
}

fn align_up(n: usize, align: usize) -> usize {
    let align = align.max(1);
    n.saturating_add(align - 1) / align * align
}

fn u16_at(bytes: &[u8], at: usize, data: Data) -> Option<u16> {
    let raw = bytes.get(at..at + 2)?;
    let b = [raw[0], raw[1]];
    Some(match data {
        Data::Little => u16::from_le_bytes(b),
        Data::Big => u16::from_be_bytes(b),
    })
}

fn u32_at(bytes: &[u8], at: usize, data: Data) -> Option<u32> {
    let raw = bytes.get(at..at + 4)?;
    let b = [raw[0], raw[1], raw[2], raw[3]];
    Some(match data {
        Data::Little => u32::from_le_bytes(b),
        Data::Big => u32::from_be_bytes(b),
    })
}

fn u64_at(bytes: &[u8], at: usize, data: Data) -> Option<u64> {
    let raw = bytes.get(at..at + 8)?;
    let b = [
        raw[0], raw[1], raw[2], raw[3], raw[4], raw[5], raw[6], raw[7],
    ];
    Some(match data {
        Data::Little => u64::from_le_bytes(b),
        Data::Big => u64::from_be_bytes(b),
    })
}

#[cfg(test)]
mod tests {
    use super::super::header::{self, Class, Type};
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

    fn phdr(buf: &mut [u8], at: usize, ty: u32, offset: u64, filesz: u64, align: u64) {
        put_u32(buf, at, ty);
        put_u64(buf, at + 8, offset);
        put_u64(buf, at + 16, offset);
        put_u64(buf, at + 32, filesz);
        put_u64(buf, at + 40, filesz);
        put_u64(buf, at + 48, align);
    }

    /// vaddr equals the file offset. One undefined printf and one defined _start.
    fn dyn_file(note: bool) -> Vec<u8> {
        let phnum: usize = if note { 3 } else { 2 };
        let mut buf = vec![0u8; 64 + phnum * 56];
        buf[0..4].copy_from_slice(&[0x7f, b'E', b'L', b'F']);
        buf[4] = 2;
        buf[5] = 1;
        buf[6] = 1;
        put_u16(&mut buf, 16, 2);
        put_u16(&mut buf, 18, 62);
        put_u32(&mut buf, 20, 1);
        put_u64(&mut buf, 24, 0x2000);
        put_u64(&mut buf, 32, 64);
        put_u16(&mut buf, 52, 64);
        put_u16(&mut buf, 54, 56);
        put_u16(&mut buf, 56, phnum as u16);

        let dyn_off = buf.len();
        buf.resize(dyn_off + 9 * 16, 0);
        let hash_off = buf.len();
        buf.resize(hash_off + 24, 0);
        put_u32(&mut buf, hash_off, 1);
        put_u32(&mut buf, hash_off + 4, 3);
        let sym_off = buf.len();
        buf.resize(sym_off + 3 * 24, 0);
        put_u32(&mut buf, sym_off + 24, 1);
        buf[sym_off + 24 + 4] = 0x10;
        put_u32(&mut buf, sym_off + 48, 8);
        buf[sym_off + 48 + 4] = 0x10;
        put_u16(&mut buf, sym_off + 48 + 6, 1);
        put_u64(&mut buf, sym_off + 48 + 8, 0x2000);
        let str_off = buf.len();
        let strings = b"\0printf\0_start\0libc.so.6\0GLIBC_2.2.5\0";
        buf.extend_from_slice(strings);
        let versym_off = buf.len();
        buf.extend_from_slice(&0u16.to_le_bytes());
        buf.extend_from_slice(&2u16.to_le_bytes());
        buf.extend_from_slice(&1u16.to_le_bytes());
        let ver_off = buf.len();
        buf.resize(ver_off + 32, 0);
        put_u16(&mut buf, ver_off, 1);
        put_u16(&mut buf, ver_off + 2, 1);
        put_u32(&mut buf, ver_off + 4, 15);
        put_u32(&mut buf, ver_off + 8, 16);
        put_u16(&mut buf, ver_off + 16 + 6, 2);
        put_u32(&mut buf, ver_off + 16 + 8, 25);

        let note_off = buf.len();
        if note {
            let notes = note_blob();
            buf.extend_from_slice(&notes);
        }
        let end = buf.len() as u64;
        phdr(&mut buf, 64, 1, 0, end, 8);
        phdr(&mut buf, 64 + 56, 2, dyn_off as u64, (9 * 16) as u64, 8);
        if note {
            let note_size = (buf.len() - note_off) as u64;
            phdr(&mut buf, 64 + 112, 4, note_off as u64, note_size, 8);
        }
        let tags = [
            (5u64, str_off as u64),
            (10, strings.len() as u64),
            (6, sym_off as u64),
            (11, 24),
            (4, hash_off as u64),
            (0x6fff_fff0, versym_off as u64),
            (0x6fff_fffe, ver_off as u64),
            (0x6fff_ffff, 1),
            (0, 0),
        ];
        for (index, (tag, val)) in tags.iter().enumerate() {
            let at = dyn_off + index * 16;
            put_u64(&mut buf, at, *tag);
            put_u64(&mut buf, at + 8, *val);
        }
        buf
    }

    fn note_blob() -> Vec<u8> {
        let mut note = vec![0u8; 36 + 40];
        put_u32(&mut note, 0, 4);
        put_u32(&mut note, 4, 16);
        put_u32(&mut note, 8, 1);
        note[12..15].copy_from_slice(b"GNU");
        put_u32(&mut note, 20, 0);
        put_u32(&mut note, 24, 3);
        put_u32(&mut note, 28, 2);
        put_u32(&mut note, 32, 0);
        put_u32(&mut note, 36, 4);
        put_u32(&mut note, 40, 16);
        put_u32(&mut note, 44, 5);
        note[48..51].copy_from_slice(b"GNU");
        put_u32(&mut note, 56, 0xc000_8002);
        put_u32(&mut note, 60, 4);
        put_u32(&mut note, 64, 2);
        note
    }

    #[test]
    fn dynsym_records_printf_at_glibc_and_omits_abi() {
        let bytes = dyn_file(false);
        let header = header::parse(&bytes).unwrap();
        let symbols = read(&bytes, &header, true);
        assert_eq!(symbols.undef, 1);
        assert_eq!(symbols.defined, 1);
        assert_eq!(symbols.undef_rows.len(), 1);
        assert_eq!(symbols.undef_rows[0].name, "printf");
        assert_eq!(
            symbols.undef_rows[0].version.as_deref(),
            Some("GLIBC_2.2.5")
        );
        assert_eq!(symbols.undef_rows[0].binding, "global");
        assert_eq!(symbols.verneed.len(), 1);
        assert_eq!(symbols.verneed[0].file, "libc.so.6");
        assert_eq!(symbols.verneed[0].need, "GLIBC_2.2.5");
        assert!(!symbols.symtab);
        assert!(!symbols.has_abi());
        assert_eq!(symbols.entry_name.as_deref(), Some("_start"));
        assert!(symbols.entry_note.contains("SHT_DYNSYM"));

        let load = load::read(&bytes, &header);
        let text = page::render_with("dyn", &header, &load, &symbols, false);
        assert!(text.contains("undef:"), "{text}");
        assert!(text.contains("name: printf"), "{text}");
        assert!(text.contains("version: GLIBC_2.2.5"), "{text}");
        assert!(text.contains("binding: global"), "{text}");
        assert!(text.contains("file: libc.so.6"), "{text}");
        assert!(text.contains("need: GLIBC_2.2.5"), "{text}");
        assert!(text.contains("symtab: false"), "{text}");
        assert!(text.contains("entry_name: _start"), "{text}");
        assert!(!text.contains("\nabi:"), "{text}");
        assert!(!text.contains("says:"), "{text}");

        let counts = read(&bytes, &header, false);
        assert!(counts.undef_rows.is_empty());
        let count_page = page::render_with("dyn", &header, &load, &counts, false);
        assert!(count_page.contains("undef: 1"), "{count_page}");
        assert!(!count_page.contains("name: printf"), "{count_page}");
    }

    #[test]
    fn abi_tag_and_property_print_only_when_present() {
        let bytes = dyn_file(true);
        let header = header::parse(&bytes).unwrap();
        let symbols = read(&bytes, &header, false);
        let tag = symbols.abi_tag.expect("abi tag");
        assert_eq!(tag.text(), "linux 3.2.0");
        assert_eq!(symbols.property, ["x86-64-v2"]);
        let load = load::read(&bytes, &header);
        let text = page::render_with("dyn", &header, &load, &symbols, false);
        assert!(text.contains("tag: linux 3.2.0"), "{text}");
        assert!(text.contains("x86-64-v2"), "{text}");
        assert!(text.contains("[NT_GNU_ABI_TAG]"), "{text}");
    }

    #[test]
    fn hostile_hash_verneed_and_sections_do_not_panic() {
        let mut bytes = dyn_file(false);
        let header = header::parse(&bytes).unwrap();
        let hash_off = header.phoff as usize + 56 * 2;
        // Dynamic starts just after the program headers. Hash nchain is inside it.
        let dyn_off = 64 + 56 * 2;
        let mut hash_at = 0usize;
        for index in 0..9 {
            let at = dyn_off + index * 16;
            let tag = u64::from_le_bytes(bytes[at..at + 8].try_into().unwrap());
            if tag == 4 {
                hash_at = u64::from_le_bytes(bytes[at + 8..at + 16].try_into().unwrap()) as usize;
            }
        }
        assert!(hash_at > hash_off);
        put_u32(&mut bytes, hash_at + 4, u32::MAX);
        let symbols = read(&bytes, &header, true);
        assert_eq!(symbols.undef, 1, "nchain is limited by the bytes that fit");

        let mut header = header.clone();
        header.shnum = u16::MAX;
        header.shentsize = 64;
        header.shoff = 0;
        header.phnum = u16::MAX;
        let _ = read(&bytes, &header, true);
        let _ = read(&[], &header, true);

        let mut cyc = dyn_file(false);
        let parsed = header::parse(&cyc).unwrap();
        let dyn_off = 64 + 56 * 2;
        let mut ver_at = 0usize;
        for index in 0..9 {
            let at = dyn_off + index * 16;
            let tag = u64::from_le_bytes(cyc[at..at + 8].try_into().unwrap());
            if tag == 0x6fff_fffe {
                ver_at = u64::from_le_bytes(cyc[at + 8..at + 16].try_into().unwrap()) as usize;
            }
            if tag == 0x6fff_ffff {
                put_u64(&mut cyc, at + 8, 100);
            }
        }
        put_u32(&mut cyc, ver_at + 12, 4);
        let symbols = read(&cyc, &parsed, true);
        assert!(symbols.verneed.len() <= 32, "{}", symbols.verneed.len());
    }

    #[test]
    fn a_static_header_has_no_abi_group() {
        let header = header::sample(Class::Elf64, Type::Exec);
        let symbols = read(&[0; 64], &header, false);
        assert!(!symbols.has_abi());
        assert!(symbols.entry_name.is_none());
        assert!(!symbols.symtab);
    }

    #[test]
    fn huge_hash_vaddr_does_not_panic() {
        let mut bytes = dyn_file(false);
        put_u64(&mut bytes, 64 + 32, u64::MAX);
        let dyn_off = 64 + 56 * 2;
        for index in 0..9 {
            let at = dyn_off + index * 16;
            let tag = u64::from_le_bytes(bytes[at..at + 8].try_into().unwrap());
            if tag == 4 {
                put_u64(&mut bytes, at + 8, u64::MAX - 3);
            }
        }
        let header = header::parse(&bytes).unwrap();
        let parsed = std::panic::catch_unwind(|| read(&bytes, &header, false));
        assert!(parsed.is_ok(), "a huge DT_HASH vaddr panicked");
        let symbols = parsed.unwrap();
        assert_eq!(symbols.undef, 0);
    }

    #[test]
    fn gnu_hash_bloom_cap_does_not_shift_buckets() {
        let bloom_n = 4097usize;
        let header_len = 64usize;
        let ph_bytes = 112usize;
        let dyn_bytes = 4 * 16;
        let gnu_off = header_len + ph_bytes + dyn_bytes;
        let bucket_off = gnu_off + 16 + bloom_n * 8;
        let chain_off = bucket_off + 4;
        let sym_off = chain_off + 4;
        let mut bytes = vec![0u8; sym_off + 48];
        bytes[0..4].copy_from_slice(&[0x7f, b'E', b'L', b'F']);
        bytes[4] = 2;
        bytes[5] = 1;
        bytes[6] = 1;
        put_u16(&mut bytes, 16, 3);
        put_u16(&mut bytes, 18, 62);
        put_u32(&mut bytes, 20, 1);
        put_u64(&mut bytes, 32, header_len as u64);
        put_u16(&mut bytes, 52, 64);
        put_u16(&mut bytes, 54, 56);
        put_u16(&mut bytes, 56, 2);
        let filesz = bytes.len() as u64;
        phdr(&mut bytes, 64, 1, 0, filesz, 8);
        phdr(
            &mut bytes,
            64 + 56,
            2,
            (header_len + ph_bytes) as u64,
            dyn_bytes as u64,
            8,
        );
        let dyn_off = header_len + ph_bytes;
        let tags = [
            (6u64, sym_off as u64),
            (11, 24),
            (0x6fff_fef5, gnu_off as u64),
            (0, 0),
        ];
        for (index, (tag, val)) in tags.iter().enumerate() {
            let at = dyn_off + index * 16;
            put_u64(&mut bytes, at, *tag);
            put_u64(&mut bytes, at + 8, *val);
        }
        put_u32(&mut bytes, gnu_off, 1);
        put_u32(&mut bytes, gnu_off + 4, 1);
        put_u32(&mut bytes, gnu_off + 8, bloom_n as u32);
        put_u32(&mut bytes, bucket_off, 1);
        put_u32(&mut bytes, chain_off, 1);
        bytes[sym_off + 24 + 4] = 0x10;
        put_u16(&mut bytes, sym_off + 24 + 6, 1);
        let header = header::parse(&bytes).unwrap();
        let symbols = read(&bytes, &header, false);
        assert_eq!(
            symbols.defined, 1,
            "bloom size 4097 must not move the buckets"
        );
    }
}
