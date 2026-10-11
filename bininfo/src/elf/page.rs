/*
---
why: This file writes one ELF header as one text page.
uses: bininfo
how: bininfo calls this file after it reads the header.
Each line has one field and one note.
This file does not read ELF bytes.
---
*/

//! One page for one ELF header. Fields sit under the ladder rungs.
//! Each line is `key: value  # note`. This file does not read ELF bytes.
//! Color is Carbonfox, and only when the caller asks for it.

use super::header::{Class, Data, Header, Type};
use super::load::{self, Load};
use super::symbols::Symbols;
use super::words::{class_name, data_name, machine_name, osabi_name, type_name};
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
    let wide = match header.class {
        Class::Elf32 => "4 bytes.",
        Class::Elf64 => "8 bytes.",
    };
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
        "elf",
        "elf, macho, or pe.",
        style::FG,
        true,
    );

    style::blank(&mut out);
    type_rung(&mut out, header, color);

    style::blank(&mut out);
    load_rung(&mut out, load, color);

    style::blank(&mut out);
    write_symbols(&mut out, symbols, color);
    if symbols.has_abi() {
        style::blank(&mut out);
        write_abi(&mut out, symbols, color);
    }

    style::blank(&mut out);
    size_rung(&mut out, header, color);

    style::blank(&mut out);
    where_rung(&mut out, header, symbols, wide, color);

    style::blank(&mut out);
    section_rung(&mut out, header, wide, color);
    out
}

fn type_rung(out: &mut String, header: &Header, color: bool) {
    style::rung(
        out,
        color,
        "type",
        "Rung 1. Magic, word size, byte order, CPU, and file type.",
        style::GREEN,
        true,
    );
    ident_field(out, header, color);
    style::field(
        out,
        color,
        2,
        "class",
        class_name(header.class),
        "1 byte. Address width.",
        style::FG,
        true,
    );
    let data_fg = match header.data {
        Data::Little => style::BLUE,
        Data::Big => style::TEAL,
    };
    style::field(
        out,
        color,
        2,
        "data",
        data_name(header.data),
        "1 byte. Byte order of every multi-byte number.",
        data_fg,
        false,
    );
    style::field(
        out,
        color,
        2,
        "ei_version",
        &header.ei_version.to_string(),
        "1 byte. 1 is current.",
        style::WHITE,
        false,
    );
    style::field(
        out,
        color,
        2,
        "osabi",
        &header.os_abi.to_string(),
        "1 byte. 0 is System V. 3 is Linux.",
        style::WHITE,
        false,
    );
    let osabi = osabi_name(header.os_abi);
    let osabi_text = style::named(osabi, u64::from(header.os_abi));
    let (fg, bold) = style::word_tone(osabi, style::WHITE, false);
    style::field(
        out,
        color,
        2,
        "osabi_name",
        &osabi_text,
        "Word for the osabi number.",
        fg,
        bold,
    );
    style::field(
        out,
        color,
        2,
        "abi_version",
        &header.abi_version.to_string(),
        "1 byte. Usually 0.",
        style::WHITE,
        false,
    );
    let (fg, bold) = type_tone(header.kind);
    style::field(
        out,
        color,
        2,
        "e_type",
        &type_name(header.kind),
        "2 bytes. NONE, REL, EXEC, DYN, or CORE. The kernel runs EXEC and DYN.",
        fg,
        bold,
    );
    style::field(
        out,
        color,
        2,
        "e_machine",
        &header.machine.to_string(),
        "2 bytes. Target CPU.",
        style::BLUE,
        true,
    );
    let machine = machine_name(header.machine);
    let machine_text = style::named(machine, u64::from(header.machine));
    let (fg, bold) = style::word_tone(machine, style::BLUE, true);
    style::field(
        out,
        color,
        2,
        "e_machine_name",
        &machine_text,
        "Word for the CPU number.",
        fg,
        bold,
    );
    style::field(
        out,
        color,
        2,
        "e_version",
        &header.version.to_string(),
        "4 bytes. 1 is current.",
        style::WHITE,
        false,
    );
    let (fg, bold) = style::number_tone(header.flags as u64, style::TEAL, false);
    style::field(
        out,
        color,
        2,
        "e_flags",
        &style::hex(u64::from(header.flags)),
        "4 bytes. CPU flags. Often 0.",
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
    let interp = load.interp.as_deref().unwrap_or("null");
    let (fg, bold) = if load.interp.is_some() {
        (style::WHITE, false)
    } else {
        (style::LABEL, false)
    };
    style::field(
        out,
        color,
        2,
        "interp",
        interp,
        "Path of the loader. null means this file starts itself.",
        fg,
        bold,
    );
    style::noted_list(
        out,
        color,
        2,
        "needed",
        &load.needed,
        claim::needed_note("elf"),
        if load.needed.is_empty() {
            style::LABEL
        } else {
            style::WHITE
        },
        false,
        crate::library::note,
    );
    claim::write_search(out, color, 2, "elf", &load.runpath, &load.rpath);
    claim::write_delay(out, color, 2, "elf", &load.delay);
    claim::write_dlopen(out, color, 2, "elf", &load.needed);
    style::field(
        out,
        color,
        2,
        "pie",
        if load.pie { "true" } else { "false" },
        "PIE bit in DT_FLAGS_1.",
        if load.pie { style::GREEN } else { style::LABEL },
        load.pie,
    );
}

fn size_rung(out: &mut String, header: &Header, color: bool) {
    style::rung(
        out,
        color,
        "sizes",
        "Rung 2. Size of this header and size of one program header.",
        style::BLUE,
        true,
    );
    style::field(
        out,
        color,
        2,
        "e_ehsize",
        &header.ehsize.to_string(),
        "2 bytes. Size of this header.",
        style::WHITE,
        false,
    );
    let (fg, bold) = style::number_tone(header.phentsize as u64, style::WHITE, false);
    style::field(
        out,
        color,
        2,
        "e_phentsize",
        &header.phentsize.to_string(),
        "2 bytes. Size of one program header.",
        fg,
        bold,
    );
    let (fg, bold) = style::number_tone(header.phnum as u64, style::WHITE, false);
    style::field(
        out,
        color,
        2,
        "e_phnum",
        &header.phnum.to_string(),
        "2 bytes. Number of program headers.",
        fg,
        bold,
    );
}

fn where_rung(out: &mut String, header: &Header, symbols: &Symbols, wide: &str, color: bool) {
    style::rung(
        out,
        color,
        "where",
        "Assumed entry address and program-header offset.",
        style::FG,
        true,
    );
    let (fg, bold) = style::number_tone(header.entry, style::FG, true);
    style::field(
        out,
        color,
        2,
        "e_entry",
        &style::hex(header.entry),
        &format!("{wide} Assumed address of the first instruction."),
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
    let (fg, bold) = style::number_tone(header.phoff, style::WHITE, false);
    style::field(
        out,
        color,
        2,
        "e_phoff",
        &header.phoff.to_string(),
        &format!("{wide} File offset of the program headers."),
        fg,
        bold,
    );
}

fn section_rung(out: &mut String, header: &Header, wide: &str, color: bool) {
    style::rung(
        out,
        color,
        "sections",
        "The kernel does not read these fields to start the program.",
        style::LABEL,
        false,
    );
    let (fg, bold) = style::number_tone(header.shoff, style::WHITE, false);
    style::field(
        out,
        color,
        2,
        "e_shoff",
        &header.shoff.to_string(),
        &format!("{wide} File offset of the section headers."),
        fg,
        bold,
    );
    style::field(
        out,
        color,
        2,
        "e_shentsize",
        &header.shentsize.to_string(),
        "2 bytes. Size of one section header.",
        style::WHITE,
        false,
    );
    style::field(
        out,
        color,
        2,
        "e_shnum",
        &header.shnum.to_string(),
        "2 bytes. Number of section headers.",
        style::WHITE,
        false,
    );
    style::field(
        out,
        color,
        2,
        "e_shstrndx",
        &header.shstrndx.to_string(),
        "2 bytes. Index of the section-name section.",
        style::WHITE,
        false,
    );
}

fn write_symbols(out: &mut String, symbols: &Symbols, color: bool) {
    style::rung(
        out,
        color,
        "symbols",
        "Counts from the dynamic symbol table.",
        style::FG,
        true,
    );
    let undef_note = "[SHT_DYNSYM] Undefined symbols. The loader binds these.";
    if symbols.rows {
        if symbols.undef_rows.is_empty() {
            style::field(
                out,
                color,
                2,
                "undef",
                "[]",
                undef_note,
                style::LABEL,
                false,
            );
        } else {
            key_line(out, color, 2, "undef", undef_note);
            for row in &symbols.undef_rows {
                let _ = writeln!(out, "    - name: {}", style::yaml_scalar(&row.name));
                let version = row.version.as_deref().unwrap_or("null");
                let _ = writeln!(out, "      version: {}", style::yaml_scalar(version));
                let _ = writeln!(out, "      binding: {}", row.binding);
            }
        }
    } else {
        let (fg, bold) = style::number_tone(u64::from(symbols.undef), style::WHITE, false);
        style::field(
            out,
            color,
            2,
            "undef",
            &symbols.undef.to_string(),
            undef_note,
            fg,
            bold,
        );
    }
    let (fg, bold) = style::number_tone(u64::from(symbols.defined), style::WHITE, false);
    style::field(
        out,
        color,
        2,
        "defined",
        &symbols.defined.to_string(),
        "[SHT_DYNSYM] Defined symbols that are not local.",
        fg,
        bold,
    );
    if !symbols.verneed.is_empty() {
        key_line(
            out,
            color,
            2,
            "verneed",
            "[DT_VERNEED] Version nodes this file requires.",
        );
        for need in &symbols.verneed {
            let _ = writeln!(out, "    - file: {}", style::yaml_scalar(&need.file));
            let _ = writeln!(out, "      need: {}", style::yaml_scalar(&need.need));
        }
    }
    let symtab_note = if symbols.symtab {
        ".symtab is present. The loader does not read it."
    } else {
        ".symtab is absent. The loader does not read it."
    };
    style::field(
        out,
        color,
        2,
        "symtab",
        if symbols.symtab { "true" } else { "false" },
        symtab_note,
        if symbols.symtab {
            style::WHITE
        } else {
            style::LABEL
        },
        false,
    );
}

fn write_abi(out: &mut String, symbols: &Symbols, color: bool) {
    style::rung(
        out,
        color,
        "abi",
        "GNU notes in this file.",
        style::FG,
        true,
    );
    if let Some(tag) = symbols.abi_tag {
        style::field(
            out,
            color,
            2,
            "tag",
            &tag.text(),
            "[NT_GNU_ABI_TAG]",
            style::WHITE,
            false,
        );
    }
    if !symbols.property.is_empty() {
        style::list_field(
            out,
            color,
            2,
            "property",
            &symbols.property,
            "[.note.gnu.property]",
            style::WHITE,
            false,
        );
    }
}

fn key_line(out: &mut String, color: bool, indent: usize, key: &str, note: &str) {
    let gap = " ".repeat(indent);
    let painted = style::paint(color, &format!("{key}:"), style::LABEL, false);
    let plain = indent + key.len() + 1;
    style::align_note(out, color, &format!("{gap}{painted}"), plain, note);
}

fn ident_field(out: &mut String, header: &Header, color: bool) {
    let shown = ident_text(&header.ident);
    let split = magic_end(&shown);
    let key = style::paint(color, "e_ident:", style::LABEL, false);
    let magic = style::paint(color, &shown[..split], style::GREEN, false);
    let rest = style::paint(color, &shown[split..], style::LABEL, false);
    let plain = 2 + "e_ident: ".len() + shown.chars().count();
    style::align_note(
        out,
        color,
        &format!("  {key} {magic}{rest}"),
        plain,
        "16 bytes. Magic[4] Class[1] Data[1] Version[1] OSABI[1] ABI[1] Pad[7].",
    );
}

fn ident_text(bytes: &[u8; 16]) -> String {
    let mut out = String::from("[");
    for (i, byte) in bytes.iter().enumerate() {
        if i > 0 {
            out.push_str(", ");
        }
        out.push_str(&byte.to_string());
    }
    out.push(']');
    out
}

fn magic_end(text: &str) -> usize {
    let mut commas = 0;
    for (i, c) in text.char_indices() {
        if c == ',' {
            commas += 1;
            if commas == 4 {
                return i;
            }
        }
    }
    text.len()
}

fn type_tone(kind: Type) -> (style::Rgb, bool) {
    match kind {
        Type::None => (style::LABEL, false),
        Type::Rel => (style::TEAL, false),
        Type::Exec => (style::GREEN, true),
        Type::Dyn => (style::CYAN, true),
        Type::Core => (style::MAGENTA, true),
        Type::Other(_) => (style::PINK, false),
    }
}

#[cfg(test)]
mod tests {
    use super::super::header::{sample, Class, Data, Type};
    use super::super::load::{Kind, Load};
    use super::*;

    fn still() -> Load {
        Load {
            kind: Kind::Static,
            interp: None,
            needed: Vec::new(),
            runpath: Vec::new(),
            rpath: Vec::new(),
            delay: Vec::new(),
            pie: false,
        }
    }

    fn hash_at(text: &str, key: &str) -> usize {
        text.lines()
            .find(|line| line.starts_with(key))
            .unwrap()
            .find('#')
            .unwrap()
    }

    #[test]
    fn notes_align_under_the_rungs() {
        let text = render(
            "exec-x86-64",
            &sample(Class::Elf64, Type::Exec),
            &still(),
            false,
        );
        assert!(text.lines().any(|line| line.starts_with("type:")), "{text}");
        assert!(
            text.lines().any(|line| line.starts_with("sizes:")),
            "{text}"
        );
        assert!(
            text.lines().any(|line| line.starts_with("where:")),
            "{text}"
        );
        assert!(
            text.lines().any(|line| line.starts_with("sections:")),
            "{text}"
        );
        assert!(text.contains("# Rung 1. Magic, word size, byte order, CPU, and file type."));
        assert!(text.contains("# Rung 2. Size of this header and size of one program header."));
        assert!(text.contains("e_type: EXEC"), "{text}");
        assert!(text.contains("e_machine_name: x86-64 (62)"), "{text}");
        assert!(text.contains("osabi_name: System V (0)"), "{text}");
        assert!(text.lines().any(|line| line.starts_with("load:")), "{text}");
        assert!(text.contains("format: elf"), "{text}");
        assert!(text.contains("kind: static"), "{text}");
        assert!(text.contains("needed: []"), "{text}");
        assert!(
            text.contains("[DT_RUNPATH] Directories the loader searches."),
            "{text}"
        );
        assert!(
            text.contains("[DT_RPATH] Old search directories."),
            "{text}"
        );
        assert!(text.contains("pie: false"), "{text}");
        assert!(!text.contains("hermetic:"), "{text}");
        assert!(!text.contains("says:"), "{text}");
        assert!(text.contains("e_entry: 0x1000"), "{text}");
        assert!(
            text.contains("# 8 bytes. Assumed address of the first instruction."),
            "{text}"
        );
        assert_eq!(hash_at(&text, "type:"), hash_at(&text, "  class:"));
        assert_eq!(hash_at(&text, "sizes:"), hash_at(&text, "  e_ehsize:"));
        let ident = text
            .lines()
            .position(|line| line.starts_with("  e_ident:"))
            .unwrap();
        let ident_line = text.lines().nth(ident).unwrap();
        assert!(ident_line.contains('#'), "{ident_line}");
        assert!(ident_line.contains("16 bytes. Magic[4]"), "{ident_line}");
        assert!(!text.contains('\u{1b}'), "{text}");
    }

    #[test]
    fn elf32_note_says_four_bytes() {
        let text = render(
            "exec-i386",
            &sample(Class::Elf32, Type::Dyn),
            &still(),
            false,
        );
        assert!(text.contains("class: ELF32"), "{text}");
        assert!(
            text.contains("# 4 bytes. Assumed address of the first instruction."),
            "{text}"
        );
    }

    #[test]
    fn null_name_and_a_hash_in_the_path() {
        let mut header = sample(Class::Elf64, Type::Exec);
        header.machine = 1;
        header.data = Data::Big;
        let text = render("a#b", &header, &still(), false);
        assert!(text.contains("data: big-endian"), "{text}");
        assert!(text.contains("e_machine_name: null"), "{text}");
        assert!(!text.contains("e_machine_name: null ("), "{text}");
        assert!(text.contains("file: \"a#b\""), "{text}");
        assert!(text.contains("# Path of this file."), "{text}");
    }

    #[test]
    fn carbonfox_colors_the_rungs() {
        let text = render(
            "exec-x86-64",
            &sample(Class::Elf64, Type::Exec),
            &still(),
            true,
        );
        assert!(text.contains("\u{1b}[1;38;2;37;190;106mtype"), "{text}");
        assert!(text.contains("\u{1b}[1;38;2;120;169;255msizes"), "{text}");
        assert!(text.contains("\u{1b}[1;38;2;37;190;106mEXEC"), "{text}");
        assert!(
            text.contains("\u{1b}[38;2;37;190;106m[127, 69, 76, 70"),
            "{text}"
        );
        assert!(text.contains("38;2;157;200;230m# "), "{text}");
        assert!(text.contains("\u{1b}[38;2;123;124;126msections"), "{text}");
        assert!(!text.contains("\u{1b}[2m"), "{text}");
    }

    #[test]
    fn needed_names_are_a_block_list_in_file_order() {
        let mut load = still();
        load.needed = vec!["libz.so".into(), "liba:b.so".into()];
        let text = render("dyn", &sample(Class::Elf64, Type::Dyn), &load, false);
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
            lines[at].contains("[DT_NEEDED] Library names the loader opens."),
            "{}",
            lines[at]
        );
        assert!(lines[at + 1].contains("    - libz.so"), "{}", lines[at + 1]);
        assert!(
            lines[at + 1].contains("zlib compression."),
            "{}",
            lines[at + 1]
        );
        assert!(lines[at + 2].contains("liba:b.so"), "{}", lines[at + 2]);
        assert!(
            lines[at + 2].contains("A shared library."),
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
            lines[at + 6].starts_with("  dlopen: false"),
            "{}",
            lines[at + 6]
        );
        assert!(lines[at + 7].starts_with("  pie:"), "{}", lines[at + 7]);
    }

    #[test]
    fn search_paths_are_block_lists_in_file_order() {
        let mut load = still();
        load.runpath = vec!["$ORIGIN/../lib".into(), "/opt/lib".into()];
        load.rpath = vec!["/old".into()];
        let text = render("dyn", &sample(Class::Elf64, Type::Dyn), &load, false);
        let lines: Vec<&str> = text.lines().collect();
        let at = lines
            .iter()
            .position(|line| line.starts_with("  runpath:"))
            .unwrap();
        assert!(
            lines[at].contains("[DT_RUNPATH] Directories the loader searches."),
            "{}",
            lines[at]
        );
        assert_eq!(lines[at + 1], "    - $ORIGIN/../lib");
        assert_eq!(lines[at + 2], "    - /opt/lib");
        assert!(
            lines[at + 3].contains("[DT_RPATH] Old search directories."),
            "{}",
            lines[at + 3]
        );
        assert_eq!(lines[at + 4], "    - /old");
        assert!(
            lines[at + 5].starts_with("  delay: []"),
            "{}",
            lines[at + 5]
        );
        assert!(
            lines[at + 6].starts_with("  dlopen: false"),
            "{}",
            lines[at + 6]
        );
        assert!(lines[at + 7].starts_with("  pie:"), "{}", lines[at + 7]);
    }

    #[test]
    fn libdl_marks_dlopen() {
        let mut load = still();
        load.needed = vec!["/lib/libdl.so.2".into()];
        let text = render("dyn", &sample(Class::Elf64, Type::Dyn), &load, false);
        assert!(text.contains("dlopen: true"), "{text}");
        assert!(
            text.contains("[DT_NEEDED] A libdl soname is present. This file can call dlopen. The call names the library later."),
            "{text}"
        );
        load.needed = vec!["libc.so.6".into()];
        let plain = render("dyn", &sample(Class::Elf64, Type::Dyn), &load, false);
        assert!(plain.contains("dlopen: false"), "{plain}");
    }

    #[test]
    fn a_common_needed_name_has_a_note() {
        let mut load = still();
        load.needed = vec!["libc.so.6".into(), "libtiny.so".into()];
        let text = render("dyn", &sample(Class::Elf64, Type::Dyn), &load, false);
        let lines: Vec<&str> = text.lines().collect();
        let at = lines
            .iter()
            .position(|line| line.starts_with("  needed:"))
            .unwrap();
        assert!(lines[at + 1].contains("libc.so.6"), "{}", lines[at + 1]);
        assert!(
            lines[at + 1].contains("The C library."),
            "{}",
            lines[at + 1]
        );
        assert_eq!(
            lines[at + 1].find('#'),
            Some(style::COLUMN + 2),
            "{}",
            lines[at + 1]
        );
        assert!(lines[at + 2].contains("libtiny.so"), "{}", lines[at + 2]);
        assert!(
            lines[at + 2].contains("A shared library."),
            "{}",
            lines[at + 2]
        );
    }
}
