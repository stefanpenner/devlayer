//! ELF file header. The first 16 bytes choose the struct and the byte order.

/// Address width. This is byte 4 of `e_ident`.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Class {
    Elf32, // 4-byte addresses. The header is 52 bytes.
    Elf64, // 8-byte addresses. The header is 64 bytes.
}

/// Byte order of every multi-byte number. This is byte 5 of `e_ident`.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Data {
    Little, // The bytes `3e 00` store the number 62.
    Big,    // The bytes `00 3e` store the number 62.
}

/// File type. This is `e_type`.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Type {
    None,       // 0. The file has no type.
    Rel,        // 1. A `.o` file. The kernel does not run it.
    Exec,       // 2. The load address is fixed.
    Dyn,        // 3. A PIE, a static-PIE, or a shared library.
    Core,       // 4. A core dump.
    Other(u16), // Any other value stored in the file.
}

/// An ELF64 header is 64 bytes. An ELF32 header is 52 bytes.
pub const HEADER_BYTES: usize = 64;

/// Decoded ELF file header.
/// `entry` is an assumed address. `phoff` and `shoff` are file offsets.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Header {
    pub ident: [u8; 16], // Bytes 0..16. Magic, class, byte order, and OS ABI.
    pub class: Class,    // ELF32 or ELF64.
    pub data: Data,      // Little-endian or big-endian.
    pub ei_version: u8,  // Byte 6. 1 is the current format.
    pub os_abi: u8,      // 0 is System V. 3 is Linux.
    pub abi_version: u8, // OS ABI version. Usually 0.
    pub kind: Type,      // `e_type`.
    pub machine: u16,    // `e_machine`. 62 is x86-64. 183 is AArch64.
    pub version: u32,    // `e_version`. 1 is the current format.
    pub entry: u64,      // `e_entry`. Assumed address of the first instruction.
    pub phoff: u64,      // `e_phoff`. File offset of the program headers.
    pub shoff: u64,      // `e_shoff`. File offset of the section headers.
    pub flags: u32,      // `e_flags`. CPU flags. Often 0.
    pub ehsize: u16,     // Byte size of this header.
    pub phentsize: u16,  // Byte size of one program header.
    pub phnum: u16,      // Number of program headers.
    pub shentsize: u16,  // Byte size of one section header.
    pub shnum: u16,      // Number of section headers.
    pub shstrndx: u16,   // Index of the section that stores section names.
}

pub fn is_elf(bytes: &[u8]) -> bool {
    bytes.len() >= 4 && bytes[..4] == [0x7f, b'E', b'L', b'F']
}

pub fn parse(bytes: &[u8]) -> Result<Header, String> {
    let ident = read_ident(bytes)?;
    let class = class(ident[4])?;
    let data = data(ident[5])?;
    decode(bytes, ident, class, data)
}

fn read_ident(bytes: &[u8]) -> Result<[u8; 16], String> {
    let magic = bytes.get(..4).ok_or("file ends before the ELF header")?;
    if magic != [0x7f, b'E', b'L', b'F'] {
        return Err("not an ELF file".into());
    }
    let raw = bytes.get(..16).ok_or("file ends before the ELF header")?;
    let mut ident = [0u8; 16];
    ident.copy_from_slice(raw);
    Ok(ident)
}

fn class(byte: u8) -> Result<Class, String> {
    match byte {
        1 => Ok(Class::Elf32),
        2 => Ok(Class::Elf64),
        _ => Err("unknown ELF class".into()),
    }
}

fn data(byte: u8) -> Result<Data, String> {
    match byte {
        1 => Ok(Data::Little),
        2 => Ok(Data::Big),
        _ => Err("unknown ELF data encoding".into()),
    }
}

fn decode(bytes: &[u8], ident: [u8; 16], class: Class, data: Data) -> Result<Header, String> {
    let need = match class {
        Class::Elf32 => 52,
        Class::Elf64 => HEADER_BYTES,
    };
    if bytes.len() < need {
        return Err("file ends before the ELF header".into());
    }
    let kind = match read_u16(bytes, 16, data)? {
        0 => Type::None,
        1 => Type::Rel,
        2 => Type::Exec,
        3 => Type::Dyn,
        4 => Type::Core,
        other => Type::Other(other),
    };
    let machine = read_u16(bytes, 18, data)?;
    let version = read_u32(bytes, 20, data)?;
    let (entry, phoff, shoff, flags_at) = match class {
        Class::Elf32 => (
            read_u32(bytes, 24, data)? as u64,
            read_u32(bytes, 28, data)? as u64,
            read_u32(bytes, 32, data)? as u64,
            36,
        ),
        Class::Elf64 => (
            read_u64(bytes, 24, data)?,
            read_u64(bytes, 32, data)?,
            read_u64(bytes, 40, data)?,
            48,
        ),
    };
    Ok(Header {
        ident,
        class,
        data,
        ei_version: ident[6],
        os_abi: ident[7],
        abi_version: ident[8],
        kind,
        machine,
        version,
        entry,
        phoff,
        shoff,
        flags: read_u32(bytes, flags_at, data)?,
        ehsize: read_u16(bytes, flags_at + 4, data)?,
        phentsize: read_u16(bytes, flags_at + 6, data)?,
        phnum: read_u16(bytes, flags_at + 8, data)?,
        shentsize: read_u16(bytes, flags_at + 10, data)?,
        shnum: read_u16(bytes, flags_at + 12, data)?,
        shstrndx: read_u16(bytes, flags_at + 14, data)?,
    })
}

fn read_u16(bytes: &[u8], at: usize, data: Data) -> Result<u16, String> {
    let s = bytes
        .get(at..at + 2)
        .ok_or("file ends before the ELF header")?;
    let b = [s[0], s[1]];
    Ok(match data {
        Data::Little => u16::from_le_bytes(b),
        Data::Big => u16::from_be_bytes(b),
    })
}

fn read_u32(bytes: &[u8], at: usize, data: Data) -> Result<u32, String> {
    let s = bytes
        .get(at..at + 4)
        .ok_or("file ends before the ELF header")?;
    let b = [s[0], s[1], s[2], s[3]];
    Ok(match data {
        Data::Little => u32::from_le_bytes(b),
        Data::Big => u32::from_be_bytes(b),
    })
}

fn read_u64(bytes: &[u8], at: usize, data: Data) -> Result<u64, String> {
    let s = bytes
        .get(at..at + 8)
        .ok_or("file ends before the ELF header")?;
    let b = [s[0], s[1], s[2], s[3], s[4], s[5], s[6], s[7]];
    Ok(match data {
        Data::Little => u64::from_le_bytes(b),
        Data::Big => u64::from_be_bytes(b),
    })
}

#[cfg(test)]
pub fn sample(class: Class, kind: Type) -> Header {
    Header {
        ident: [0x7f, b'E', b'L', b'F', 2, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0],
        class,
        data: Data::Little,
        ei_version: 1,
        os_abi: 0,
        abi_version: 0,
        kind,
        machine: 62,
        version: 1,
        entry: 0x1000,
        phoff: 64,
        shoff: 0,
        flags: 0,
        ehsize: if class == Class::Elf32 { 52 } else { 64 },
        phentsize: 56,
        phnum: 1,
        shentsize: 0,
        shnum: 0,
        shstrndx: 0,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

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

    fn ident(class: u8, data: u8) -> [u8; 16] {
        let mut id = [0u8; 16];
        id[0..4].copy_from_slice(&[0x7f, b'E', b'L', b'F']);
        id[4] = class;
        id[5] = data;
        id[6] = 1;
        id
    }

    fn elf64(data: Data) -> Vec<u8> {
        let mut b = vec![0u8; HEADER_BYTES];
        let class_byte = 2;
        let data_byte = match data {
            Data::Little => 1,
            Data::Big => 2,
        };
        b[..16].copy_from_slice(&ident(class_byte, data_byte));
        put_u16(&mut b, 16, 2, data);
        put_u16(&mut b, 18, 62, data);
        put_u32(&mut b, 20, 1, data);
        put_u64(&mut b, 24, 0x1000, data);
        put_u64(&mut b, 32, 64, data);
        put_u32(&mut b, 48, 0, data);
        put_u16(&mut b, 52, 64, data);
        put_u16(&mut b, 54, 56, data);
        put_u16(&mut b, 56, 1, data);
        b
    }

    #[test]
    fn elf64_little_exec() {
        let h = parse(&elf64(Data::Little)).unwrap();
        assert_eq!(h.class, Class::Elf64);
        assert_eq!(h.data, Data::Little);
        assert_eq!(h.ei_version, 1);
        assert_eq!(h.kind, Type::Exec);
        assert_eq!(h.machine, 62);
        assert_eq!(h.entry, 0x1000);
        assert_eq!(h.phoff, 64);
        assert_eq!(h.ehsize, 64);
        assert_eq!(h.phentsize, 56);
        assert_eq!(h.phnum, 1);
    }

    #[test]
    fn elf64_big_endian_swaps_multibyte_fields() {
        let h = parse(&elf64(Data::Big)).unwrap();
        assert_eq!(h.data, Data::Big);
        assert_eq!(h.kind, Type::Exec);
        assert_eq!(h.machine, 62);
        assert_eq!(h.entry, 0x1000);
    }

    #[test]
    fn elf32_uses_four_byte_addresses() {
        let mut b = vec![0u8; 52];
        b[..16].copy_from_slice(&ident(1, 1));
        put_u16(&mut b, 16, 3, Data::Little);
        put_u16(&mut b, 18, 3, Data::Little);
        put_u32(&mut b, 20, 1, Data::Little);
        put_u32(&mut b, 24, 0x1111_1111, Data::Little);
        put_u32(&mut b, 28, 0x2222_2222, Data::Little);
        put_u16(&mut b, 40, 52, Data::Little);
        let h = parse(&b).unwrap();
        assert_eq!(h.class, Class::Elf32);
        assert_eq!(h.kind, Type::Dyn);
        assert_eq!(h.machine, 3);
        assert_eq!(h.entry, 0x1111_1111);
        assert_eq!(h.phoff, 0x2222_2222);
        assert_eq!(h.ehsize, 52);
    }

    #[test]
    fn rejects_bad_files() {
        assert_eq!(parse(&[]).unwrap_err(), "file ends before the ELF header");
        assert_eq!(parse(b"nope").unwrap_err(), "not an ELF file");
        assert_eq!(parse(&[0; 16]).unwrap_err(), "not an ELF file");
        let mut bad_class = vec![0u8; HEADER_BYTES];
        bad_class[..16].copy_from_slice(&ident(9, 1));
        assert_eq!(parse(&bad_class).unwrap_err(), "unknown ELF class");
        let mut bad_data = vec![0u8; HEADER_BYTES];
        bad_data[..16].copy_from_slice(&ident(2, 9));
        assert_eq!(parse(&bad_data).unwrap_err(), "unknown ELF data encoding");
        let mut short = elf64(Data::Little);
        short.truncate(40);
        assert_eq!(
            parse(&short).unwrap_err(),
            "file ends before the ELF header"
        );
    }

    #[test]
    fn hostile_bytes_do_not_panic() {
        let mut samples: Vec<Vec<u8>> = vec![
            vec![],
            b"\x7fELF".to_vec(),
            b"\x7fELF\x02\x01".to_vec(),
            vec![0xff; 80],
            elf64(Data::Little),
            elf64(Data::Big),
        ];
        samples[4][16] = 0xff;
        samples[4][17] = 0xff;
        for n in 0..80 {
            samples.push(vec![0u8; n]);
            samples.push(vec![0xff; n]);
            let mut marked = vec![0u8; n];
            if n >= 4 {
                marked[..4].copy_from_slice(b"\x7fELF");
            }
            samples.push(marked);
        }
        for s in samples {
            let parsed = std::panic::catch_unwind(|| parse(&s));
            assert!(parsed.is_ok(), "parse panicked");
        }
    }
}
