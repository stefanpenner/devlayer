//! PE header. `MZ` points at `PE`, and the optional magic chooses the word size.

/// Address width. A COFF object has no optional header.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Class {
    Pe32,     // Magic 0x10b. 4-byte image base.
    Pe32Plus, // Magic 0x20b. 8-byte image base.
    Coff,     // A `.obj` file. No DOS header and no optional header.
}

/// Decoded COFF and optional header.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Header {
    pub class: Class,
    pub dos: u16,                 // 0x5a4d is MZ. 0 for a COFF object.
    pub lfanew: u32,              // File offset of the PE signature. 0 for COFF.
    pub machine: u16,             // 0x8664 is x86-64. 0x14c is x86.
    pub sections: u16,            // Number of sections.
    pub symoff: u32,              // File offset of the COFF symbol table.
    pub nsyms: u32,               // Number of COFF symbols.
    pub opt_size: u16,            // SizeOfOptionalHeader.
    pub characteristics: u16,     // EXE is 0x2. DLL is 0x2000.
    pub subsystem: u16,           // 3 is the console.
    pub dll_characteristics: u16, // DYNAMIC_BASE is 0x40.
    pub entry: u32,               // AddressOfEntryPoint. An RVA.
    pub image_base: u64,          // Assumed base address.
    pub size_of_headers: u32,     // Bytes before the first section body.
    pub opt_off: u32,             // File offset of the optional header.
    pub section_off: u32,         // File offset of the section table.
}

pub fn is_pe(bytes: &[u8]) -> bool {
    is_mz(bytes) || is_coff(bytes)
}

pub fn parse(bytes: &[u8]) -> Result<Header, String> {
    if is_mz(bytes) {
        return parse_image(bytes);
    }
    if is_coff(bytes) {
        return parse_coff(bytes, 0, 0, true);
    }
    Err("not a PE file".into())
}

fn is_mz(bytes: &[u8]) -> bool {
    matches!(bytes.get(..2), Some([0x4d, 0x5a] | [0x5a, 0x4d]))
}

fn is_coff(bytes: &[u8]) -> bool {
    if bytes.len() < 20 || is_mz(bytes) {
        return false;
    }
    let Some(machine) = u16_at(bytes, 0) else {
        return false;
    };
    let Some(opt) = u16_at(bytes, 16) else {
        return false;
    };
    opt == 0 && known_machine(machine)
}

fn known_machine(machine: u16) -> bool {
    matches!(
        machine,
        0x14c | 0x1c0 | 0x1c4 | 0x1f0 | 0x1f1 | 0x200 | 0x8664 | 0xaa64 | 0x5032 | 0x5064
    )
}

fn parse_image(bytes: &[u8]) -> Result<Header, String> {
    if bytes.len() < 0x40 {
        return Err("file ends before the PE header".into());
    }
    let lfanew = u32_at(bytes, 0x3c).ok_or("file ends before the PE header")?;
    let at = usize::try_from(lfanew).map_err(|_| "file ends before the PE header".to_string())?;
    let sig = at.checked_add(4).ok_or("file ends before the PE header")?;
    if sig > bytes.len() || &bytes[at..sig] != b"PE\0\0" {
        return Err(if sig > bytes.len() {
            "file ends before the PE header"
        } else {
            "DOS file has no PE signature"
        }
        .into());
    }
    let dos = u16_at(bytes, 0).unwrap_or(0);
    parse_coff(bytes, sig, lfanew, false).map(|mut header| {
        header.dos = dos;
        header
    })
}

fn parse_coff(bytes: &[u8], coff: usize, lfanew: u32, coff_only: bool) -> Result<Header, String> {
    if coff
        .checked_add(20)
        .map(|n| n > bytes.len())
        .unwrap_or(true)
    {
        return Err("file ends before the PE header".into());
    }
    let machine = u16_at(bytes, coff).unwrap_or(0);
    let sections = u16_at(bytes, coff + 2).unwrap_or(0);
    let symoff = u32_at(bytes, coff + 8).unwrap_or(0);
    let nsyms = u32_at(bytes, coff + 12).unwrap_or(0);
    let opt_size = u16_at(bytes, coff + 16).unwrap_or(0);
    let characteristics = u16_at(bytes, coff + 18).unwrap_or(0);
    let opt_off =
        u32::try_from(coff + 20).map_err(|_| "file ends before the PE header".to_string())?;
    let section_off = opt_off
        .checked_add(opt_size as u32)
        .ok_or("file ends before the PE header")?;
    if coff_only {
        return Ok(Header {
            class: Class::Coff,
            dos: 0,
            lfanew,
            machine,
            sections,
            symoff,
            nsyms,
            opt_size,
            characteristics,
            subsystem: 0,
            dll_characteristics: 0,
            entry: 0,
            image_base: 0,
            size_of_headers: 0,
            opt_off,
            section_off,
        });
    }
    let class = match u16_at(bytes, opt_off as usize) {
        Some(0x10b) => Class::Pe32,
        Some(0x20b) => Class::Pe32Plus,
        Some(_) => return Err("unknown PE magic".into()),
        None => return Err("file ends before the PE header".into()),
    };
    Ok(Header {
        class,
        dos: 0,
        lfanew,
        machine,
        sections,
        symoff,
        nsyms,
        opt_size,
        characteristics,
        subsystem: opt_u16(bytes, opt_off, opt_size, 68),
        dll_characteristics: opt_u16(bytes, opt_off, opt_size, 70),
        entry: opt_u32(bytes, opt_off, opt_size, 16),
        image_base: match class {
            Class::Pe32 => opt_u32(bytes, opt_off, opt_size, 28) as u64,
            Class::Pe32Plus => opt_u64(bytes, opt_off, opt_size, 24),
            Class::Coff => 0,
        },
        size_of_headers: opt_u32(bytes, opt_off, opt_size, 60),
        opt_off,
        section_off,
    })
}

fn opt_u16(bytes: &[u8], opt: u32, size: u16, rel: u16) -> u16 {
    if rel.saturating_add(2) > size {
        return 0;
    }
    u16_at(bytes, opt as usize + rel as usize).unwrap_or(0)
}

fn opt_u32(bytes: &[u8], opt: u32, size: u16, rel: u16) -> u32 {
    if rel.saturating_add(4) > size {
        return 0;
    }
    u32_at(bytes, opt as usize + rel as usize).unwrap_or(0)
}

fn opt_u64(bytes: &[u8], opt: u32, size: u16, rel: u16) -> u64 {
    if rel.saturating_add(8) > size {
        return 0;
    }
    u64_at(bytes, opt as usize + rel as usize).unwrap_or(0)
}

pub(crate) fn u16_at(bytes: &[u8], at: usize) -> Option<u16> {
    let s = bytes.get(at..at + 2)?;
    Some(u16::from_le_bytes([s[0], s[1]]))
}

pub(crate) fn u32_at(bytes: &[u8], at: usize) -> Option<u32> {
    let s = bytes.get(at..at + 4)?;
    Some(u32::from_le_bytes([s[0], s[1], s[2], s[3]]))
}

pub(crate) fn u64_at(bytes: &[u8], at: usize) -> Option<u64> {
    let s = bytes.get(at..at + 8)?;
    Some(u64::from_le_bytes([
        s[0], s[1], s[2], s[3], s[4], s[5], s[6], s[7],
    ]))
}

#[cfg(test)]
mod tests {
    use super::*;

    fn pe(class_magic: u16, dllc: u16) -> Vec<u8> {
        let mut b = vec![0u8; 0x40 + 4 + 20 + 128];
        b[0] = b'M';
        b[1] = b'Z';
        b[0x3c] = 0x40;
        b[0x40..0x44].copy_from_slice(b"PE\0\0");
        b[0x44] = 0x64;
        b[0x45] = 0x86;
        b[0x44 + 16] = 128;
        b[0x44 + 18] = 0x02;
        let opt = 0x44 + 20;
        b[opt] = class_magic as u8;
        b[opt + 1] = (class_magic >> 8) as u8;
        b[opt + 16] = 0x00;
        b[opt + 17] = 0x10;
        if class_magic == 0x20b {
            b[opt + 24] = 0x00;
            b[opt + 25] = 0x00;
            b[opt + 26] = 0x00;
            b[opt + 27] = 0x40;
            b[opt + 28] = 0x01;
        } else {
            b[opt + 28] = 0x00;
            b[opt + 29] = 0x00;
            b[opt + 30] = 0x40;
        }
        b[opt + 70] = dllc as u8;
        b[opt + 71] = (dllc >> 8) as u8;
        b[opt + 68] = 3;
        b
    }

    #[test]
    fn pe32_plus_reads_the_image_base() {
        let h = parse(&pe(0x20b, 0x160)).unwrap();
        assert_eq!(h.class, Class::Pe32Plus);
        assert_eq!(h.machine, 0x8664);
        assert_eq!(h.entry, 0x1000);
        assert_eq!(h.image_base, 0x1_4000_0000);
        assert_eq!(h.subsystem, 3);
        assert_eq!(h.dll_characteristics, 0x160);
        assert_eq!(h.characteristics & 0x2, 0x2);
    }

    #[test]
    fn pe32_uses_a_four_byte_image_base() {
        let h = parse(&pe(0x10b, 0x40)).unwrap();
        assert_eq!(h.class, Class::Pe32);
        assert_eq!(h.image_base, 0x40_0000);
    }

    #[test]
    fn coff_object_has_no_optional_header() {
        let mut b = vec![0u8; 20];
        b[0] = 0x64;
        b[1] = 0x86;
        b[2] = 1;
        let h = parse(&b).unwrap();
        assert_eq!(h.class, Class::Coff);
        assert_eq!(h.sections, 1);
        assert_eq!(h.dos, 0);
        assert!(is_pe(&b));
    }

    #[test]
    fn rejects_a_dos_file_and_a_bad_magic() {
        assert_eq!(parse(b"MZ").unwrap_err(), "file ends before the PE header");
        let mut dos = vec![0u8; 0x80];
        dos[0] = b'M';
        dos[1] = b'Z';
        dos[0x3c] = 0x40;
        assert_eq!(parse(&dos).unwrap_err(), "DOS file has no PE signature");
        let mut bad = pe(0x20b, 0);
        let opt = 0x44 + 20;
        bad[opt] = 0;
        bad[opt + 1] = 0;
        assert_eq!(parse(&bad).unwrap_err(), "unknown PE magic");
    }

    #[test]
    fn hostile_offsets_do_not_panic() {
        let mut samples = vec![vec![], b"MZ".to_vec(), pe(0x20b, 0), pe(0x10b, 0)];
        let mut huge = pe(0x20b, 0);
        huge[0x3c..0x40].copy_from_slice(&u32::MAX.to_le_bytes());
        samples.push(huge);
        for n in 0..80 {
            samples.push(vec![0u8; n]);
        }
        for s in samples {
            assert!(std::panic::catch_unwind(|| parse(&s)).is_ok());
            assert!(std::panic::catch_unwind(|| is_pe(&s)).is_ok());
        }
    }
}
