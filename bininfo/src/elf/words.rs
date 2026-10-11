//! Words for ELF numbers. This file does not read ELF bytes.

use super::header::{Class, Data, Type};

pub(crate) fn class_name(class: Class) -> &'static str {
    match class {
        Class::Elf32 => "ELF32",
        Class::Elf64 => "ELF64",
    }
}

pub(crate) fn data_name(data: Data) -> &'static str {
    match data {
        Data::Little => "little-endian",
        Data::Big => "big-endian",
    }
}

pub(crate) fn type_name(kind: Type) -> String {
    match kind {
        Type::None => "NONE".into(),
        Type::Rel => "REL".into(),
        Type::Exec => "EXEC".into(),
        Type::Dyn => "DYN".into(),
        Type::Core => "CORE".into(),
        Type::Other(n) => n.to_string(),
    }
}

pub(crate) fn machine_name(machine: u16) -> Option<&'static str> {
    Some(match machine {
        3 => "x86",
        8 => "MIPS",
        20 => "PowerPC",
        21 => "PowerPC64",
        40 => "ARM",
        62 => "x86-64",
        183 => "AArch64",
        243 => "RISC-V",
        _ => return None,
    })
}

pub(crate) fn osabi_name(os_abi: u8) -> Option<&'static str> {
    Some(match os_abi {
        0 => "System V",
        3 => "Linux",
        _ => return None,
    })
}
