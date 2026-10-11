/*
---
why: This file writes one PE header as one text page.
uses: bininfo
how: bininfo calls this file after it reads the header.
Each line has one field and one note.
This file does not read PE bytes.
---
*/

//! One page for one PE header. Fields sit under the same rungs as ELF.
//! This file does not read PE bytes.

use super::header::{Class, Header};
use super::load::{self, Load};
use super::symbols::{self, Symbols};
use super::words::{class_name, machine_name, magic_name, subsystem_name};
use crate::claim;
use crate::style;
use std::fmt::Write;

#[cfg(test)]
pub fn render(file: &str, header: &Header, load: &Load, color: bool) -> String {
    render_with(file, header, load, &Symbols::empty(), color)
}

pub fn render_with(
    file: &str,
    header: &Header,
    load: &Load,
    symbols: &Symbols,
    color: bool,
) -> String {
    let mut out = String::new();
    style::field(
        &mut out,
        color,
        0,
        "file",
        file,
        "Path of this file.",
        style::WHITE,
        false,
    );
    style::field(
        &mut out,
        color,
        0,
        "format",
        "pe",
        "elf, macho, or pe.",
        style::FG,
        true,
    );
    style::blank(&mut out);
    type_rung(&mut out, header, symbols, color);
    style::blank(&mut out);
    load_rung(&mut out, load, color);
    style::blank(&mut out);
    write_symbols(&mut out, symbols, color);
    style::blank(&mut out);
    size_rung(&mut out, header, color);
    style::blank(&mut out);
    where_rung(&mut out, header, symbols, color);
    style::blank(&mut out);
    section_rung(&mut out, header, color);
    out
}

fn type_rung(out: &mut String, header: &Header, symbols: &Symbols, color: bool) {
    style::rung(
        out,
        color,
        "type",
        "Rung 1. Magic, word size, byte order, CPU, and file type.",
        style::GREEN,
        true,
    );
    let magic = magic_name(header.dos);
    let (fg, bold) = if magic == "null" {
        (style::LABEL, false)
    } else {
        (style::GREEN, false)
    };
    style::field(
        out,
        color,
        2,
        "magic",
        magic,
        "2 bytes. DOS mark.",
        fg,
        bold,
    );
    let sig = if header.class == Class::Coff {
        "null"
    } else {
        "PE"
    };
    let (fg, bold) = if sig == "null" {
        (style::LABEL, false)
    } else {
        (style::GREEN, true)
    };
    style::field(
        out,
        color,
        2,
        "signature",
        sig,
        "4 bytes. PE mark.",
        fg,
        bold,
    );
    style::field(
        out,
        color,
        2,
        "class",
        class_name(header.class),
        "Address width.",
        style::FG,
        true,
    );
    style::field(
        out,
        color,
        2,
        "data",
        "little-endian",
        "Byte order of every multi-byte number.",
        style::BLUE,
        false,
    );
    style::field(
        out,
        color,
        2,
        "machine",
        &header.machine.to_string(),
        "2 bytes. Target CPU.",
        style::BLUE,
        true,
    );
    let cpu = machine_name(header.machine);
    let cpu_text = style::named(cpu, u64::from(header.machine));
    let (fg, bold) = style::word_tone(cpu, style::BLUE, true);
    style::field(
        out,
        color,
        2,
        "machine_name",
        &cpu_text,
        "Word for the CPU number.",
        fg,
        bold,
    );
    let (fg, bold) = style::number_tone(header.characteristics as u64, style::TEAL, false);
    style::field(
        out,
        color,
        2,
        "characteristics",
        &style::hex(u64::from(header.characteristics)),
        "2 bytes. EXE is 0x2. DLL is 0x2000.",
        fg,
        bold,
    );
    style::field(
        out,
        color,
        2,
        "subsystem",
        &header.subsystem.to_string(),
        "2 bytes. 3 is the console.",
        style::WHITE,
        false,
    );
    let sub = subsystem_name(header.subsystem);
    let sub_text = style::named(sub, u64::from(header.subsystem));
    let (fg, bold) = style::word_tone(sub, style::WHITE, false);
    style::field(
        out,
        color,
        2,
        "subsystem_name",
        &sub_text,
        "Word for the subsystem number.",
        fg,
        bold,
    );
    if let Some(os) = symbols.os {
        style::field(
            out,
            color,
            2,
            "os_version",
            &symbols::pair_text(os),
            "Operating system version in the optional header.",
            style::WHITE,
            false,
        );
    }
    if let Some(pair) = symbols.subsystem {
        style::field(
            out,
            color,
            2,
            "subsystem_version",
            &symbols::pair_text(pair),
            "Subsystem version in the optional header.",
            style::WHITE,
            false,
        );
    }
    let (fg, bold) = style::number_tone(header.dll_characteristics as u64, style::TEAL, false);
    style::field(
        out,
        color,
        2,
        "dll_characteristics",
        &style::hex(u64::from(header.dll_characteristics)),
        "2 bytes. DYNAMIC_BASE is 0x40.",
        fg,
        bold,
    );
}

fn load_rung(out: &mut String, load: &Load, color: bool) {
    style::rung(
        out,
        color,
        "load",
        "How this file starts, and which files it must open.",
        style::FG,
        true,
    );
    let (fg, bold) = style::kind_tone(load.kind);
    style::field(
        out,
        color,
        2,
        "kind",
        load::kind_name(load.kind),
        load::kind_note(load.kind),
        fg,
        bold,
    );
    style::field(
        out,
        color,
        2,
        "interp",
        "null",
        "Path of the loader. null means this file starts itself.",
        style::LABEL,
        false,
    );
    style::noted_list(
        out,
        color,
        2,
        "needed",
        &load.needed,
        claim::needed_note("pe"),
        if load.needed.is_empty() {
            style::LABEL
        } else {
            style::WHITE
        },
        false,
        crate::library::note,
    );
    claim::write_search(out, color, 2, "pe", &load.runpath, &load.rpath);
    claim::write_delay(out, color, 2, "pe", &load.delay);
    claim::write_dlopen(out, color, 2, "pe", &load.needed);
    style::field(
        out,
        color,
        2,
        "pie",
        if load.pie { "true" } else { "false" },
        "DYNAMIC_BASE bit in DllCharacteristics.",
        if load.pie { style::GREEN } else { style::LABEL },
        load.pie,
    );
}

fn size_rung(out: &mut String, header: &Header, color: bool) {
    style::rung(
        out,
        color,
        "sizes",
        "Rung 2. Size of the optional header and the section count.",
        style::BLUE,
        true,
    );
    style::field(
        out,
        color,
        2,
        "size_of_optional_header",
        &header.opt_size.to_string(),
        "2 bytes. Size of the optional header.",
        style::WHITE,
        false,
    );
    let (fg, bold) = style::number_tone(header.sections as u64, style::WHITE, false);
    style::field(
        out,
        color,
        2,
        "number_of_sections",
        &header.sections.to_string(),
        "2 bytes. Number of sections.",
        fg,
        bold,
    );
    let (fg, bold) = style::number_tone(header.size_of_headers as u64, style::WHITE, false);
    style::field(
        out,
        color,
        2,
        "size_of_headers",
        &header.size_of_headers.to_string(),
        "4 bytes. Bytes before the section bodies.",
        fg,
        bold,
    );
}

fn where_rung(out: &mut String, header: &Header, symbols: &Symbols, color: bool) {
    let wide = if header.class == Class::Pe32 {
        "4 bytes."
    } else {
        "8 bytes."
    };
    style::rung(
        out,
        color,
        "where",
        "Entry RVA and assumed image base.",
        style::FG,
        true,
    );
    let (fg, bold) = style::number_tone(header.entry as u64, style::FG, true);
    style::field(
        out,
        color,
        2,
        "entry",
        &style::hex(u64::from(header.entry)),
        "4 bytes. RVA of the first instruction.",
        fg,
        bold,
    );
    let shown = symbols.entry_name.as_deref().unwrap_or("null");
    let (fg, bold) = style::word_tone(symbols.entry_name.as_deref(), style::FG, true);
    style::field(
        out,
        color,
        2,
        "entry_name",
        shown,
        symbols.entry_note,
        fg,
        bold,
    );
    let (fg, bold) = style::number_tone(header.image_base, style::FG, true);
    style::field(
        out,
        color,
        2,
        "image_base",
        &style::hex(header.image_base),
        &format!("{wide} Assumed base address."),
        fg,
        bold,
    );
    style::field(
        out,
        color,
        2,
        "e_lfanew",
        &header.lfanew.to_string(),
        "4 bytes. File offset of the PE signature.",
        style::WHITE,
        false,
    );
}

fn write_symbols(out: &mut String, symbols: &Symbols, color: bool) {
    style::rung(
        out,
        color,
        "symbols",
        "Import and export directories.",
        style::FG,
        true,
    );
    let import_note = "[IMAGE_IMPORT_DESCRIPTOR]";
    if symbols.rows {
        if symbols.imports.is_empty() {
            style::field(
                out,
                color,
                2,
                "import",
                "[]",
                import_note,
                style::LABEL,
                false,
            );
        } else {
            key_line(out, color, 2, "import", import_note);
            for row in &symbols.imports {
                let _ = writeln!(out, "    - dll: {}", style::yaml_scalar(&row.dll));
                if let Some(name) = &row.name {
                    let _ = writeln!(out, "      name: {}", style::yaml_scalar(name));
                }
                if let Some(hint) = row.hint {
                    let _ = writeln!(out, "      hint: {hint}");
                }
                if let Some(ordinal) = row.ordinal {
                    let _ = writeln!(out, "      ordinal: {ordinal}");
                }
            }
        }
    } else {
        let (fg, bold) = style::number_tone(u64::from(symbols.import_count), style::WHITE, false);
        style::field(
            out,
            color,
            2,
            "import",
            &symbols.import_count.to_string(),
            import_note,
            fg,
            bold,
        );
    }
    let (fg, bold) = style::number_tone(u64::from(symbols.ordinal_count), style::WHITE, false);
    style::field(
        out,
        color,
        2,
        "ordinal",
        &symbols.ordinal_count.to_string(),
        "Rows in that directory with no name.",
        fg,
        bold,
    );
    let (fg, bold) = style::number_tone(u64::from(symbols.export_count), style::WHITE, false);
    style::field(
        out,
        color,
        2,
        "export",
        &symbols.export_count.to_string(),
        "[IMAGE_EXPORT_DIRECTORY] NumberOfNames.",
        fg,
        bold,
    );
    style::field(
        out,
        color,
        2,
        "coff_symbols",
        &symbols.coff_symbols.to_string(),
        "The loader does not read this table.",
        if symbols.coff_symbols == 0 {
            style::LABEL
        } else {
            style::WHITE
        },
        false,
    );
}

fn key_line(out: &mut String, color: bool, indent: usize, key: &str, note: &str) {
    let gap = " ".repeat(indent);
    let painted = style::paint(color, &format!("{key}:"), style::LABEL, false);
    let plain = indent + key.len() + 1;
    style::align_note(out, color, &format!("{gap}{painted}"), plain, note);
}

fn section_rung(out: &mut String, header: &Header, color: bool) {
    style::rung(
        out,
        color,
        "sections",
        "The loader does not read the COFF symbol table to start the program.",
        style::LABEL,
        false,
    );
    let (fg, bold) = style::number_tone(header.symoff as u64, style::WHITE, false);
    style::field(
        out,
        color,
        2,
        "pointer_to_symbol_table",
        &header.symoff.to_string(),
        "4 bytes. File offset of the COFF symbols.",
        fg,
        bold,
    );
    style::field(
        out,
        color,
        2,
        "number_of_symbols",
        &header.nsyms.to_string(),
        "4 bytes. Number of COFF symbols.",
        style::WHITE,
        false,
    );
}

#[cfg(test)]
mod tests {
    use super::super::header::{Class, Header};
    use super::super::load::{Kind, Load};
    use super::*;

    fn exe() -> Header {
        Header {
            class: Class::Pe32Plus,
            dos: 0x5a4d,
            lfanew: 64,
            machine: 0x8664,
            sections: 3,
            symoff: 0,
            nsyms: 0,
            opt_size: 240,
            characteristics: 0x22,
            subsystem: 3,
            dll_characteristics: 0x160,
            entry: 0x1000,
            image_base: 0x1_4000_0000,
            size_of_headers: 1024,
            opt_off: 0,
            section_off: 0,
        }
    }

    fn still() -> Load {
        Load {
            kind: Kind::StaticPie,
            interp: None,
            needed: Vec::new(),
            runpath: Vec::new(),
            rpath: Vec::new(),
            delay: Vec::new(),
            pie: true,
        }
    }

    #[test]
    fn notes_use_the_same_rungs() {
        let text = render("exec.exe", &exe(), &still(), false);
        for key in ["type:", "load:", "sizes:", "where:", "sections:"] {
            assert!(text.lines().any(|line| line.starts_with(key)), "{text}");
        }
        assert!(text.contains("format: pe"), "{text}");
        assert!(text.contains("class: PE32+"), "{text}");
        assert!(text.contains("machine_name: x86-64 (34404)"), "{text}");
        assert!(text.contains("subsystem_name: console (3)"), "{text}");
        assert!(text.contains("kind: static-pie"), "{text}");
        assert!(
            text.contains("[none] PE has no search-path record."),
            "{text}"
        );
        assert!(!text.contains("hermetic:"), "{text}");
        assert!(!text.contains("says:"), "{text}");
        assert!(
            text.contains("# DYNAMIC_BASE bit in DllCharacteristics."),
            "{text}"
        );
        assert!(text.contains("image_base: 0x140000000"), "{text}");
        assert!(!text.contains('\u{1b}'), "{text}");
    }

    #[test]
    fn needed_names_are_a_block_list_in_file_order() {
        let mut load = still();
        load.needed = vec!["b.dll".into(), "a#b.dll".into()];
        let text = render("dyn.exe", &exe(), &load, false);
        assert!(!text.contains("needed: ["), "{text}");
        let lines: Vec<&str> = text.lines().collect();
        let at = lines
            .iter()
            .position(|line| line.starts_with("  needed:"))
            .unwrap();
        assert_eq!(
            lines[at].find('#'),
            Some(style::COLUMN + 2),
            "{}",
            lines[at]
        );
        assert!(
            lines[at].contains("[IMAGE_IMPORT_DESCRIPTOR] Library names the loader opens."),
            "{}",
            lines[at]
        );
        assert!(lines[at + 1].contains("b.dll"), "{}", lines[at + 1]);
        assert!(
            lines[at + 1].contains("A Windows library."),
            "{}",
            lines[at + 1]
        );
        assert!(lines[at + 2].contains("a#b.dll"), "{}", lines[at + 2]);
        assert!(
            lines[at + 2].contains("A Windows library."),
            "{}",
            lines[at + 2]
        );
        assert!(
            lines[at + 3].starts_with("  runpath: []"),
            "{}",
            lines[at + 3]
        );
        assert!(
            lines[at + 4].starts_with("  rpath: []"),
            "{}",
            lines[at + 4]
        );
        assert!(
            lines[at + 5].starts_with("  delay: []"),
            "{}",
            lines[at + 5]
        );
        assert!(
            lines[at + 6]
                .contains("[none] kernel32 is common. That name does not prove LoadLibrary."),
            "{}",
            lines[at + 6]
        );
        assert!(lines[at + 7].starts_with("  pie:"), "{}", lines[at + 7]);
    }

    #[test]
    fn delay_names_are_a_block_list_in_file_order() {
        let mut load = still();
        load.delay = vec!["late.dll".into(), "other.dll".into()];
        let text = render("dyn.exe", &exe(), &load, false);
        let lines: Vec<&str> = text.lines().collect();
        let at = lines
            .iter()
            .position(|line| line.starts_with("  delay:"))
            .unwrap();
        assert!(
            lines[at].contains("[ImgDelayDescr] DLL names the loader opens on the first call."),
            "{}",
            lines[at]
        );
        assert!(lines[at + 1].contains("late.dll"), "{}", lines[at + 1]);
        assert!(
            lines[at + 1].contains("A Windows library."),
            "{}",
            lines[at + 1]
        );
        assert!(lines[at + 2].contains("other.dll"), "{}", lines[at + 2]);
        assert!(
            lines[at + 2].contains("A Windows library."),
            "{}",
            lines[at + 2]
        );
        assert!(
            lines[at + 3].starts_with("  dlopen: null"),
            "{}",
            lines[at + 3]
        );
    }

    #[test]
    fn a_common_needed_dll_has_a_note() {
        let mut load = still();
        load.needed = vec!["KERNEL32.dll".into(), "tiny.dll".into()];
        let text = render("dyn.exe", &exe(), &load, false);
        let lines: Vec<&str> = text.lines().collect();
        let at = lines
            .iter()
            .position(|line| line.starts_with("  needed:"))
            .unwrap();
        assert!(lines[at + 1].contains("KERNEL32.dll"), "{}", lines[at + 1]);
        assert!(
            lines[at + 1].contains("files, processes, and memory."),
            "{}",
            lines[at + 1]
        );
        assert!(lines[at + 2].contains("tiny.dll"), "{}", lines[at + 2]);
        assert!(
            lines[at + 2].contains("A Windows library."),
            "{}",
            lines[at + 2]
        );
    }

    #[test]
    fn a_common_delay_dll_has_the_same_note() {
        let mut load = still();
        load.delay = vec!["KERNEL32.dll".into()];
        let text = render("dyn.exe", &exe(), &load, false);
        assert!(text.contains("files, processes, and memory."), "{text}");
    }
}
