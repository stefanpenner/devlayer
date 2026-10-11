//! Words for PE numbers. This file does not read PE bytes.

use super::header::Class;

pub(crate) fn class_name(class: Class) -> &'static str {
    match class {
        Class::Pe32 => "PE32",
        Class::Pe32Plus => "PE32+",
        Class::Coff => "COFF",
    }
}

pub(crate) fn machine_name(machine: u16) -> Option<&'static str> {
    match machine {
        0x14c => Some("x86"),
        0x8664 => Some("x86-64"),
        0x1c0 | 0x1c4 => Some("ARM"),
        0xaa64 => Some("AArch64"),
        0x1f0 | 0x1f1 => Some("PowerPC"),
        0x5032 => Some("RISC-V"),
        0x5064 => Some("RISC-V"),
        _ => None,
    }
}

pub(crate) fn subsystem_name(subsystem: u16) -> Option<&'static str> {
    match subsystem {
        1 => Some("native"),
        2 => Some("gui"),
        3 => Some("console"),
        10 => Some("efi"),
        _ => None,
    }
}

pub(crate) fn magic_name(dos: u16) -> &'static str {
    match dos {
        0x5a4d => "MZ",
        0x4d5a => "ZM",
        _ => "null",
    }
}
