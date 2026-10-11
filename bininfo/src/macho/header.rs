//! Mach-O header. The first 4 bytes choose the word size and the byte order.

/// Word size, or a fat wrapper around more than one Mach-O.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Class {
    Mach32, // 4-byte addresses. The header is 28 bytes.
    Mach64, // 8-byte addresses. The header is 32 bytes.
    Fat,    // Big-endian slice list. One slice header is 20 bytes.
    Fat64,  // One slice header is 32 bytes.
}

impl Class {
    pub fn header_bytes(self) -> u32 {
        match self {
            Class::Mach32 => 28,
            Class::Mach64 => 32,
            Class::Fat | Class::Fat64 => 8,
        }
    }

    pub fn arch_bytes(self) -> u32 {
        match self {
            Class::Fat64 => 32,
            Class::Fat => 20,
            _ => 0,
        }
    }
}

/// Byte order of every multi-byte number.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Data {
    Little,
    Big,
}

/// `filetype` in the Mach-O header.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum FileType {
    Object,   // 1. A `.o` file.
    Execute,  // 2. A program.
    FvmLib,   // 3.
    Core,     // 4. A core dump.
    Preload,  // 5.
    Dylib,    // 6. A shared library.
    Dylinker, // 7. The loader itself.
    Bundle,   // 8. A bundle.
    Other(u32),
}

/// One thin Mach-O.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Thin {
    pub class: Class,
    pub data: Data,
    pub magic: u32,      // 0xfeedface or 0xfeedfacf.
    pub cputype: u32,    // 0x0100000c is AArch64. 0x01000007 is x86-64.
    pub cpusubtype: u32, // CPU variant.
    pub filetype: FileType,
    pub ncmds: u32,      // Number of load commands.
    pub sizeofcmds: u32, // Byte size of all load commands.
    pub flags: u32,      // MH_PIE is 0x200000.
}

/// One slice inside a fat file.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Arch {
    pub cputype: u32,
    pub cpusubtype: u32,
    pub offset: u64, // File offset of that Mach-O.
    pub size: u64,   // Byte size of that Mach-O.
}

/// A fat wrapper. Each slice opens on its own.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Fat {
    pub class: Class,
    pub data: Data,
    pub magic: u32,
    pub nfat_arch: u32,
    pub arches: Vec<Arch>,
}

/// A thin Mach-O or a fat wrapper.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Image {
    Thin(Thin),
    Fat(Fat),
}

const ARCH_CAP: u32 = 32;

pub fn is_macho(bytes: &[u8]) -> bool {
    thin_magic(bytes).is_some() || fat_magic(bytes).is_some()
}

pub fn parse(bytes: &[u8]) -> Result<Image, String> {
    if let Some((class, data)) = thin_magic(bytes) {
        return Ok(Image::Thin(read_thin(bytes, class, data)?));
    }
    if let Some((data, fat64)) = fat_magic(bytes) {
        return Ok(Image::Fat(read_fat(bytes, data, fat64)?));
    }
    Err("not a Mach-O file".into())
}

fn thin_magic(bytes: &[u8]) -> Option<(Class, Data)> {
    match bytes.get(..4)? {
        [0xce, 0xfa, 0xed, 0xfe] => Some((Class::Mach32, Data::Little)),
        [0xfe, 0xed, 0xfa, 0xce] => Some((Class::Mach32, Data::Big)),
        [0xcf, 0xfa, 0xed, 0xfe] => Some((Class::Mach64, Data::Little)),
        [0xfe, 0xed, 0xfa, 0xcf] => Some((Class::Mach64, Data::Big)),
        _ => None,
    }
}

fn fat_magic(bytes: &[u8]) -> Option<(Data, bool)> {
    if bytes.len() < 8 {
        return None;
    }
    let (data, fat64) = match bytes.get(..4)? {
        [0xca, 0xfe, 0xba, 0xbe] => (Data::Big, false),
        [0xca, 0xfe, 0xba, 0xbf] => (Data::Big, true),
        [0xbe, 0xba, 0xfe, 0xca] => (Data::Little, false),
        [0xbf, 0xba, 0xfe, 0xca] => (Data::Little, true),
        _ => return None,
    };
    let n = u32_at(bytes, 4, data)?;
    // cafebabe is also a Java class file. A Java version is at least 45.
    if n == 0 || n > ARCH_CAP {
        return None;
    }
    Some((data, fat64))
}

fn read_thin(bytes: &[u8], class: Class, data: Data) -> Result<Thin, String> {
    let need = class.header_bytes() as usize;
    if bytes.len() < need {
        return Err("file ends before the Mach-O header".into());
    }
    Ok(Thin {
        class,
        data,
        magic: u32_at(bytes, 0, data).unwrap_or(0),
        cputype: u32_at(bytes, 4, data).unwrap_or(0),
        cpusubtype: u32_at(bytes, 8, data).unwrap_or(0),
        filetype: file_type(u32_at(bytes, 12, data).unwrap_or(0)),
        ncmds: u32_at(bytes, 16, data).unwrap_or(0),
        sizeofcmds: u32_at(bytes, 20, data).unwrap_or(0),
        flags: u32_at(bytes, 24, data).unwrap_or(0),
    })
}

fn read_fat(bytes: &[u8], data: Data, fat64: bool) -> Result<Fat, String> {
    let class = if fat64 { Class::Fat64 } else { Class::Fat };
    let nfat = u32_at(bytes, 4, data).unwrap_or(0);
    let width = class.arch_bytes() as usize;
    let mut arches = Vec::new();
    let mut at = 8usize;
    for _ in 0..nfat.min(ARCH_CAP) {
        if at + width > bytes.len() {
            break;
        }
        let (offset, size, step) = if fat64 {
            (
                u64_at(bytes, at + 8, data).unwrap_or(0),
                u64_at(bytes, at + 16, data).unwrap_or(0),
                32,
            )
        } else {
            (
                u32_at(bytes, at + 8, data).unwrap_or(0) as u64,
                u32_at(bytes, at + 12, data).unwrap_or(0) as u64,
                20,
            )
        };
        arches.push(Arch {
            cputype: u32_at(bytes, at, data).unwrap_or(0),
            cpusubtype: u32_at(bytes, at + 4, data).unwrap_or(0),
            offset,
            size,
        });
        at += step;
    }
    Ok(Fat {
        class,
        data,
        magic: u32_at(bytes, 0, data).unwrap_or(0),
        nfat_arch: nfat,
        arches,
    })
}

fn file_type(value: u32) -> FileType {
    match value {
        1 => FileType::Object,
        2 => FileType::Execute,
        3 => FileType::FvmLib,
        4 => FileType::Core,
        5 => FileType::Preload,
        6 => FileType::Dylib,
        7 => FileType::Dylinker,
        8 => FileType::Bundle,
        other => FileType::Other(other),
    }
}

pub(crate) fn u32_at(bytes: &[u8], at: usize, data: Data) -> Option<u32> {
    let s = bytes.get(at..at + 4)?;
    let b = [s[0], s[1], s[2], s[3]];
    Some(match data {
        Data::Little => u32::from_le_bytes(b),
        Data::Big => u32::from_be_bytes(b),
    })
}

pub(crate) fn u64_at(bytes: &[u8], at: usize, data: Data) -> Option<u64> {
    let s = bytes.get(at..at + 8)?;
    let b = [s[0], s[1], s[2], s[3], s[4], s[5], s[6], s[7]];
    Some(match data {
        Data::Little => u64::from_le_bytes(b),
        Data::Big => u64::from_be_bytes(b),
    })
}

#[cfg(test)]
pub fn exec64() -> Image {
    Image::Thin(Thin {
        class: Class::Mach64,
        data: Data::Little,
        magic: 0xfeed_facf,
        cputype: 0x0100_000c,
        cpusubtype: 0,
        filetype: FileType::Execute,
        ncmds: 0,
        sizeofcmds: 0,
        flags: 0,
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    fn put_u32(buf: &mut [u8], at: usize, v: u32, data: Data) {
        let b = match data {
            Data::Little => v.to_le_bytes(),
            Data::Big => v.to_be_bytes(),
        };
        buf[at..at + 4].copy_from_slice(&b);
    }

    fn thin(class: Class, data: Data, filetype: u32, flags: u32) -> Vec<u8> {
        let n = class.header_bytes() as usize;
        let mut b = vec![0u8; n];
        let magic: [u8; 4] = match (class, data) {
            (Class::Mach64, Data::Little) => [0xcf, 0xfa, 0xed, 0xfe],
            (Class::Mach64, Data::Big) => [0xfe, 0xed, 0xfa, 0xcf],
            (Class::Mach32, Data::Little) => [0xce, 0xfa, 0xed, 0xfe],
            (Class::Mach32, Data::Big) => [0xfe, 0xed, 0xfa, 0xce],
            _ => [0, 0, 0, 0],
        };
        b[..4].copy_from_slice(&magic);
        put_u32(&mut b, 4, 0x0100_0007, data);
        put_u32(&mut b, 12, filetype, data);
        put_u32(&mut b, 24, flags, data);
        b
    }

    #[test]
    fn macho64_little_execute() {
        let image = parse(&thin(Class::Mach64, Data::Little, 2, 0x200000)).unwrap();
        let Image::Thin(h) = image else { panic!("fat") };
        assert_eq!(h.class, Class::Mach64);
        assert_eq!(h.data, Data::Little);
        assert_eq!(h.magic, 0xfeed_facf);
        assert_eq!(h.cputype, 0x0100_0007);
        assert_eq!(h.filetype, FileType::Execute);
        assert_eq!(h.flags, 0x200000);
    }

    #[test]
    fn big_endian_swaps_the_numbers() {
        let image = parse(&thin(Class::Mach64, Data::Big, 6, 0)).unwrap();
        let Image::Thin(h) = image else { panic!("fat") };
        assert_eq!(h.data, Data::Big);
        assert_eq!(h.cputype, 0x0100_0007);
        assert_eq!(h.filetype, FileType::Dylib);
    }

    #[test]
    fn macho32_header_is_28_bytes() {
        let image = parse(&thin(Class::Mach32, Data::Little, 1, 0)).unwrap();
        let Image::Thin(h) = image else { panic!("fat") };
        assert_eq!(h.class, Class::Mach32);
        assert_eq!(h.filetype, FileType::Object);
        assert_eq!(h.class.header_bytes(), 28);
    }

    #[test]
    fn fat_lists_one_slice_and_ignores_java() {
        let mut fat = vec![0u8; 28];
        fat[..4].copy_from_slice(&[0xca, 0xfe, 0xba, 0xbe]);
        fat[7] = 1;
        fat[8..12].copy_from_slice(&0x0100_000cu32.to_be_bytes());
        fat[16..20].copy_from_slice(&0x1000u32.to_be_bytes());
        fat[20..24].copy_from_slice(&0x200u32.to_be_bytes());
        let image = parse(&fat).unwrap();
        let Image::Fat(h) = image else { panic!("thin") };
        assert_eq!(h.nfat_arch, 1);
        assert_eq!(h.arches.len(), 1);
        assert_eq!(h.arches[0].cputype, 0x0100_000c);
        assert_eq!(h.arches[0].offset, 0x1000);
        assert_eq!(h.arches[0].size, 0x200);

        let mut class = vec![0xca, 0xfe, 0xba, 0xbe, 0, 0, 0, 52];
        class.extend_from_slice(&[0; 16]);
        assert!(!is_macho(&class));
    }

    #[test]
    fn rejects_a_short_header() {
        assert_eq!(parse(&[]).unwrap_err(), "not a Mach-O file");
        let mut short = thin(Class::Mach64, Data::Little, 2, 0);
        short.truncate(16);
        assert_eq!(
            parse(&short).unwrap_err(),
            "file ends before the Mach-O header"
        );
    }

    #[test]
    fn hostile_headers_do_not_panic() {
        let mut samples = vec![
            vec![],
            vec![0xcf, 0xfa, 0xed, 0xfe],
            thin(Class::Mach64, Data::Little, 2, 0),
            vec![0xff; 64],
        ];
        for n in 0..40 {
            samples.push(vec![0u8; n]);
            let mut marked = vec![0u8; n];
            if n >= 4 {
                marked[..4].copy_from_slice(&[0xcf, 0xfa, 0xed, 0xfe]);
            }
            samples.push(marked);
        }
        for s in samples {
            assert!(std::panic::catch_unwind(|| parse(&s)).is_ok());
            assert!(std::panic::catch_unwind(|| is_macho(&s)).is_ok());
        }
    }
}
