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

/// Mach-O `cputype` constant. The page word stays in `cpu_name`.
pub(crate) fn cpu_const(cputype: u32) -> &'static str {
    match cputype {
        7 => "CPU_TYPE_X86",
        0x0100_0007 => "CPU_TYPE_X86_64",
        12 => "CPU_TYPE_ARM",
        0x0100_000c => "CPU_TYPE_ARM64",
        0x0200_000c => "CPU_TYPE_ARM64_32",
        18 => "CPU_TYPE_POWERPC",
        0x0100_0012 => "CPU_TYPE_POWERPC64",
        _ => "unknown",
    }
}

/// Mach-O `cpusubtype` constant. A capability bit in the high byte is `unknown`.
pub(crate) fn subtype_const(cputype: u32, cpusubtype: u32) -> &'static str {
    match (cputype, cpusubtype) {
        (0x0100_000c, 0) => "CPU_SUBTYPE_ARM64_ALL",
        (0x0100_000c, 1) => "CPU_SUBTYPE_ARM64_V8",
        (0x0100_000c, 2) => "CPU_SUBTYPE_ARM64E",
        (0x0100_000c, 3) => "CPU_SUBTYPE_ARM64_X1",
        (0x0100_000c, 12) => "CPU_SUBTYPE_ARM64E_X1",
        (0x0200_000c, 0) => "CPU_SUBTYPE_ARM64_32_ALL",
        (0x0200_000c, 1) => "CPU_SUBTYPE_ARM64_32_V8",
        (0x0100_0007, 3) => "CPU_SUBTYPE_X86_64_ALL",
        (0x0100_0007, 8) => "CPU_SUBTYPE_X86_64_H",
        _ => "unknown",
    }
}

/// Set `flags` bits, low bit first. A bit with no name is its hex value.
pub(crate) fn flag_names(flags: u32) -> Vec<String> {
    const BITS: &[(u32, &str)] = &[
        (0x1, "MH_NOUNDEFS"),
        (0x2, "MH_INCRLINK"),
        (0x4, "MH_DYLDLINK"),
        (0x8, "MH_BINDATLOAD"),
        (0x10, "MH_PREBOUND"),
        (0x20, "MH_SPLIT_SEGS"),
        (0x40, "MH_LAZY_INIT"),
        (0x80, "MH_TWOLEVEL"),
        (0x100, "MH_FORCE_FLAT"),
        (0x200, "MH_NOMULTIDEFS"),
        (0x400, "MH_NOFIXPREBINDING"),
        (0x800, "MH_PREBINDABLE"),
        (0x1000, "MH_ALLMODSBOUND"),
        (0x2000, "MH_SUBSECTIONS_VIA_SYMBOLS"),
        (0x4000, "MH_CANONICAL"),
        (0x8000, "MH_WEAK_DEFINES"),
        (0x1_0000, "MH_BINDS_TO_WEAK"),
        (0x2_0000, "MH_ALLOW_STACK_EXECUTION"),
        (0x4_0000, "MH_ROOT_SAFE"),
        (0x8_0000, "MH_SETUID_SAFE"),
        (0x10_0000, "MH_NO_REEXPORTED_DYLIBS"),
        (0x20_0000, "MH_PIE"),
        (0x40_0000, "MH_DEAD_STRIPPABLE_DYLIB"),
        (0x80_0000, "MH_HAS_TLV_DESCRIPTORS"),
        (0x100_0000, "MH_NO_HEAP_EXECUTION"),
        (0x200_0000, "MH_APP_EXTENSION_SAFE"),
        (0x400_0000, "MH_NLIST_OUTOFSYNC_WITH_DYLDINFO"),
        (0x800_0000, "MH_SIM_SUPPORT"),
        (0x8000_0000, "MH_DYLIB_IN_CACHE"),
    ];
    let mut out = Vec::new();
    let mut left = flags;
    for (bit, name) in BITS {
        if flags & bit != 0 {
            out.push((*name).to_string());
            left &= !bit;
        }
    }
    if left != 0 {
        out.push(format!("0x{left:X}"));
    }
    out
}

/// Mach-O `filetype` constant.
pub(crate) fn file_type_name(ty: FileType) -> &'static str {
    match ty {
        FileType::Object => "MH_OBJECT",
        FileType::Execute => "MH_EXECUTE",
        FileType::FvmLib => "MH_FVMLIB",
        FileType::Core => "MH_CORE",
        FileType::Preload => "MH_PRELOAD",
        FileType::Dylib => "MH_DYLIB",
        FileType::Dylinker => "MH_DYLINKER",
        FileType::Bundle => "MH_BUNDLE",
        FileType::Other(_) => "MH_OTHER",
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
    use super::{flag_names, subtype_const, universal_word};

    #[test]
    fn header_flags_0x200004_are_dyldlink_and_pie() {
        assert_eq!(
            flag_names(0x0020_0004),
            vec!["MH_DYLDLINK".to_string(), "MH_PIE".to_string()]
        );
        assert_eq!(flag_names(0x2000_0000), vec!["0x20000000".to_string()]);
    }

    #[test]
    fn a_capability_bit_is_not_arm64_all() {
        assert_eq!(subtype_const(0x0100_000c, 0), "CPU_SUBTYPE_ARM64_ALL");
        assert_eq!(subtype_const(0x0100_000c, 0x8000_0000), "unknown");
    }

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
