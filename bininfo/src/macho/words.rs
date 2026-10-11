//! Words for Mach-O numbers. This file does not read Mach-O bytes.

use super::header::{Class, Data, FileType};

pub(crate) fn class_name(class: Class) -> &'static str {
    match class {
        Class::Mach32 => "MH32",
        Class::Mach64 => "MH64",
        Class::Fat => "FAT",
        Class::Fat64 => "FAT64",
    }
}

pub(crate) fn data_name(data: Data) -> &'static str {
    match data {
        Data::Little => "little-endian",
        Data::Big => "big-endian",
    }
}

pub(crate) fn cpu_name(cputype: u32) -> Option<&'static str> {
    match cputype {
        7 => Some("x86"),
        0x0100_0007 => Some("x86-64"),
        12 => Some("ARM"),
        0x0100_000c => Some("AArch64"),
        0x0200_000c => Some("ARM64_32"),
        18 => Some("PowerPC"),
        0x0100_0012 => Some("PowerPC64"),
        _ => None,
    }
}

/// Low 8 bits of `cpusubtype`. The high bits are capability flags.
pub(crate) fn subtype_name(cputype: u32, cpusubtype: u32) -> Option<&'static str> {
    match (cputype, cpusubtype & 0xff) {
        (0x0100_000c, 0) => Some("ARM64_ALL"),
        (0x0100_000c, 1) => Some("ARM64_V8"),
        (0x0100_000c, 2) => Some("ARM64E"),
        (0x0100_000c, 3) => Some("ARM64_X1"),
        (0x0100_000c, 12) => Some("ARM64E_X1"),
        (0x0200_000c, 0) => Some("ARM64_32_ALL"),
        (0x0200_000c, 1) => Some("ARM64_32_V8"),
        (0x0100_0007, 3) => Some("X86_64_ALL"),
        (0x0100_0007, 8) => Some("X86_64_H"),
        _ => None,
    }
}

/// `universal2` has an x86-64 slice and an AArch64 slice.
/// Any other non-empty fat slice list is `universal`.
pub(crate) fn universal_word(cputypes: impl IntoIterator<Item = u32>) -> Option<&'static str> {
    let mut any = false;
    let mut intel64 = false;
    let mut apple = false;
    for cpu in cputypes {
        any = true;
        match cpu {
            0x0100_0007 => intel64 = true,
            0x0100_000c => apple = true,
            _ => {}
        }
    }
    if !any {
        None
    } else if intel64 && apple {
        Some("universal2")
    } else {
        Some("universal")
    }
}

/// Low 8 bits 2 and 12 are the arm64e family. The high bits are capability flags.
pub(crate) fn is_arm64e(cputype: u32, cpusubtype: u32) -> bool {
    cputype == 0x0100_000c && matches!(cpusubtype & 0xff, 2 | 12)
}

pub(crate) fn platform_name(platform: u32) -> Option<&'static str> {
    match platform {
        1 => Some("macos"),
        2 => Some("ios"),
        3 => Some("tvos"),
        4 => Some("watchos"),
        5 => Some("bridgeos"),
        6 => Some("maccatalyst"),
        7 => Some("iossim"),
        8 => Some("tvossim"),
        9 => Some("watchossim"),
        10 => Some("driverkit"),
        11 => Some("visionos"),
        12 => Some("visionossim"),
        _ => None,
    }
}

/// OS family for a minos compare. A simulator keeps the OS it simulates.
pub(crate) fn platform_family(platform: u32) -> &'static str {
    match platform {
        1 | 6 | 10 => "macos",
        2 | 7 => "ios",
        3 | 8 => "tvos",
        4 | 9 => "watchos",
        5 => "bridgeos",
        11 | 12 => "visionos",
        _ => "",
    }
}

pub(crate) fn file_name(kind: FileType) -> String {
    match kind {
        FileType::Object => "OBJECT".into(),
        FileType::Execute => "EXECUTE".into(),
        FileType::FvmLib => "FVMLIB".into(),
        FileType::Core => "CORE".into(),
        FileType::Preload => "PRELOAD".into(),
        FileType::Dylib => "DYLIB".into(),
        FileType::Dylinker => "DYLINKER".into(),
        FileType::Bundle => "BUNDLE".into(),
        FileType::Other(n) => n.to_string(),
    }
}

#[cfg(test)]
mod tests {
    use super::universal_word;

    #[test]
    fn universal2_pairs_x86_64_with_aarch64() {
        assert_eq!(
            universal_word([0x0100_0007, 0x0100_000c]),
            Some("universal2")
        );
        assert_eq!(
            universal_word([0x0100_000c, 0x0100_0007]),
            Some("universal2")
        );
        assert_eq!(
            universal_word([0x0100_0007, 0x0100_000c, 0x0100_000c]),
            Some("universal2")
        );
    }

    #[test]
    fn any_other_fat_file_is_universal() {
        assert_eq!(
            universal_word([0x0100_000c, 0x0100_000c]),
            Some("universal")
        );
        assert_eq!(
            universal_word([0x0100_0007, 0x0200_000c]),
            Some("universal")
        );
        assert_eq!(universal_word([7, 0x0100_000c]), Some("universal"));
        assert_eq!(universal_word([0x0100_0007]), Some("universal"));
        assert_eq!(universal_word([18, 7]), Some("universal"));
    }

    #[test]
    fn an_empty_slice_list_has_no_universal_word() {
        let none: [u32; 0] = [];
        assert_eq!(universal_word(none), None);
    }
}
