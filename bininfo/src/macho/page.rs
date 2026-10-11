/*
---
why: This file writes one Mach-O header as one text page.
uses: bininfo
how: bininfo calls this file after it reads the header.
Each line has one field and one note.
A needed row shows the imports that library supplies.
This file does not read Mach-O bytes.
---
*/

//! One page for one Mach-O header. Fields sit under the same rungs as ELF.
//! This file does not read Mach-O bytes.

use super::header::{Fat, FileType, Image, Thin};
use super::load::{self, Commands, Slice};
use super::symbols;
use super::words::{
    class_name, cpu_name, data_name, file_name, platform_name, subtype_name, universal_word,
};
use crate::claim;
use crate::style;
use std::fmt::Write;

pub fn render(
    file: &str,
    image: &Image,
    commands: &Commands,
    slices: &[Slice],
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
        "macho",
        "elf, macho, or pe.",
        style::FG,
        true,
    );
    style::blank(&mut out);
    match image {
        Image::Thin(thin) => thin_page(&mut out, thin, commands, color),
        Image::Fat(fat) => fat_page(&mut out, fat, commands, slices, color),
    }
    out
}

fn thin_page(out: &mut String, thin: &Thin, commands: &Commands, color: bool) {
    type_rung(out, thin, color);
    style::blank(out);
    load_rung(
        out,
        commands,
        color,
        "How this file starts, and which files it must open.",
        false,
    );
    style::blank(out);
    write_extra(out, commands, color, 0, false);
    style::blank(out);
    size_rung(out, thin, color);
    style::blank(out);
    where_rung(out, thin, commands, color);
    style::blank(out);
    section_rung(out, commands, color);
}

fn fat_page(out: &mut String, fat: &Fat, commands: &Commands, slices: &[Slice], color: bool) {
    style::rung(
        out,
        color,
        "type",
        "Rung 1. Magic, word size, byte order, and the slice list.",
        style::GREEN,
        true,
    );
    style::field(
        out,
        color,
        2,
        "magic",
        &style::hex(u64::from(fat.magic)),
        "4 bytes. Mach-O mark.",
        style::GREEN,
        false,
    );
    style::field(
        out,
        color,
        2,
        "class",
        class_name(fat.class),
        "A fat file holds more than one Mach-O.",
        style::FG,
        true,
    );
    let word = universal_word(slices.iter().map(|slice| slice.cputype));
    let (fg, bold) = style::word_tone(word, style::BLUE, true);
    style::field(
        out,
        color,
        2,
        "universal",
        word.unwrap_or("null"),
        "universal2 pairs an x86-64 slice with an AArch64 slice. Any other fat file is universal.",
        fg,
        bold,
    );
    style::field(
        out,
        color,
        2,
        "data",
        data_name(fat.data),
        "Byte order of every multi-byte number.",
        style::BLUE,
        false,
    );
    style::field(
        out,
        color,
        2,
        "nfat_arch",
        &fat.nfat_arch.to_string(),
        "4 bytes. Number of slices.",
        style::WHITE,
        false,
    );
    style::field(
        out,
        color,
        2,
        "header_size",
        &fat.class.header_bytes().to_string(),
        "Bytes in this header.",
        style::WHITE,
        false,
    );
    style::field(
        out,
        color,
        2,
        "arch_size",
        &fat.class.arch_bytes().to_string(),
        "Bytes in one slice header.",
        style::WHITE,
        false,
    );
    style::blank(out);
    let shared = load::shared_load(slices).is_some();
    let note = if slices.is_empty() {
        "This fat header names no slice."
    } else if shared {
        "Each slice is one Mach-O. The load below matches every slice."
    } else {
        "These slices do not share one load. Read each slice."
    };
    style::rung(out, color, "slices", note, style::FG, true);
    for slice in slices {
        write_slice(out, slice, color);
    }
    if shared {
        style::blank(out);
        load_rung(
            out,
            commands,
            color,
            "Every slice has these load facts.",
            true,
        );
    }
}

fn write_slice(out: &mut String, slice: &Slice, color: bool) {
    let cpu = style::named(cpu_name(slice.cputype), u64::from(slice.cputype));
    let (fg, bold) = style::word_tone(cpu_name(slice.cputype), style::BLUE, true);
    dash_field(
        out,
        color,
        "cputype_name",
        &cpu,
        "Word for the CPU number.",
        fg,
        bold,
    );
    style::field(
        out,
        color,
        4,
        "cputype",
        &slice.cputype.to_string(),
        "4 bytes. Target CPU.",
        style::BLUE,
        true,
    );
    style::field(
        out,
        color,
        4,
        "cpusubtype",
        &slice.cpusubtype.to_string(),
        "4 bytes. CPU variant.",
        style::WHITE,
        false,
    );
    let sub_word = subtype_name(slice.cputype, slice.cpusubtype);
    let sub = style::named(sub_word, u64::from(slice.cpusubtype));
    let (fg, bold) = style::word_tone(sub_word, style::BLUE, true);
    style::field(
        out,
        color,
        4,
        "cpusubtype_name",
        &sub,
        "Word for the CPU variant. ARM64E and ARM64E_X1 share the AArch64 type.",
        fg,
        bold,
    );
    let (fg, bold) = style::number_tone(slice.offset, style::WHITE, false);
    style::field(
        out,
        color,
        4,
        "offset",
        &style::hex(slice.offset),
        "File offset of this slice.",
        fg,
        bold,
    );
    style::field(
        out,
        color,
        4,
        "size",
        &slice.size.to_string(),
        "Byte size of this slice.",
        style::WHITE,
        false,
    );
    match &slice.thin {
        Some(thin) => {
            let (fg, bold) = file_tone(thin.filetype);
            style::field(
                out,
                color,
                4,
                "filetype",
                &file_name(thin.filetype),
                "4 bytes. MH_EXECUTE, MH_DYLIB, or MH_OBJECT.",
                fg,
                bold,
            );
            let (fg, bold) = style::number_tone(u64::from(thin.flags), style::TEAL, false);
            style::field(
                out,
                color,
                4,
                "flags",
                &style::hex(u64::from(thin.flags)),
                "4 bytes. Header flags. MH_PIE is 0x200000.",
                fg,
                bold,
            );
        }
        None => {
            style::field(
                out,
                color,
                4,
                "filetype",
                "null",
                "This slice does not open.",
                style::LABEL,
                false,
            );
            style::field(
                out,
                color,
                4,
                "flags",
                "0x0",
                "4 bytes. Header flags. MH_PIE is 0x200000.",
                style::LABEL,
                false,
            );
        }
    }
    write_load_fields(out, &slice.commands, color, 4, false);
    write_extra(out, &slice.commands, color, 4, true);
}

fn dash_field(
    out: &mut String,
    color: bool,
    key: &str,
    value: &str,
    note: &str,
    fg: style::Rgb,
    bold: bool,
) {
    let plain = 4 + key.len() + 2 + value.chars().count();
    let key = style::paint(color, &format!("{key}:"), style::LABEL, false);
    let value = style::paint(color, value, fg, bold);
    style::align_note(out, color, &format!("  - {key} {value}"), plain, note);
}

fn type_rung(out: &mut String, thin: &Thin, color: bool) {
    style::rung(
        out,
        color,
        "type",
        "Rung 1. Magic, word size, byte order, CPU, and file type.",
        style::GREEN,
        true,
    );
    style::field(
        out,
        color,
        2,
        "magic",
        &style::hex(u64::from(thin.magic)),
        "4 bytes. Mach-O mark.",
        style::GREEN,
        false,
    );
    style::field(
        out,
        color,
        2,
        "class",
        class_name(thin.class),
        "Address width.",
        style::FG,
        true,
    );
    let data_fg = match thin.data {
        super::header::Data::Little => style::BLUE,
        super::header::Data::Big => style::TEAL,
    };
    style::field(
        out,
        color,
        2,
        "data",
        data_name(thin.data),
        "Byte order of every multi-byte number.",
        data_fg,
        false,
    );
    style::field(
        out,
        color,
        2,
        "cputype",
        &thin.cputype.to_string(),
        "4 bytes. Target CPU.",
        style::BLUE,
        true,
    );
    let cpu = cpu_name(thin.cputype);
    let cpu_text = style::named(cpu, u64::from(thin.cputype));
    let (fg, bold) = style::word_tone(cpu, style::BLUE, true);
    style::field(
        out,
        color,
        2,
        "cputype_name",
        &cpu_text,
        "Word for the CPU number.",
        fg,
        bold,
    );
    style::field(
        out,
        color,
        2,
        "cpusubtype",
        &thin.cpusubtype.to_string(),
        "4 bytes. CPU variant.",
        style::WHITE,
        false,
    );
    let sub_word = subtype_name(thin.cputype, thin.cpusubtype);
    let sub = style::named(sub_word, u64::from(thin.cpusubtype));
    let (fg, bold) = style::word_tone(sub_word, style::BLUE, true);
    style::field(
        out,
        color,
        2,
        "cpusubtype_name",
        &sub,
        "Word for the CPU variant. ARM64E and ARM64E_X1 share the AArch64 type.",
        fg,
        bold,
    );
    let (fg, bold) = file_tone(thin.filetype);
    style::field(
        out,
        color,
        2,
        "filetype",
        &file_name(thin.filetype),
        "4 bytes. MH_EXECUTE, MH_DYLIB, or MH_OBJECT.",
        fg,
        bold,
    );
    let (fg, bold) = style::number_tone(thin.flags as u64, style::TEAL, false);
    style::field(
        out,
        color,
        2,
        "flags",
        &style::hex(u64::from(thin.flags)),
        "4 bytes. Header flags. MH_PIE is 0x200000.",
        fg,
        bold,
    );
}

fn load_rung(out: &mut String, commands: &Commands, color: bool, note: &str, names_only: bool) {
    style::rung(out, color, "load", note, style::FG, true);
    write_load_fields(out, commands, color, 2, names_only);
}

const NEEDED_NOTE: &str = "[LC_LOAD_DYLIB] One row is one load command. No cmd means load.";

fn write_load_fields(
    out: &mut String,
    commands: &Commands,
    color: bool,
    indent: usize,
    names_only: bool,
) {
    let (fg, bold) = style::kind_tone(commands.load.kind);
    style::field(
        out,
        color,
        indent,
        "kind",
        load::kind_name(commands.load.kind),
        load::kind_note(commands.load.kind),
        fg,
        bold,
    );
    let interp = commands.load.interp.as_deref().unwrap_or("null");
    let (fg, bold) = if commands.load.interp.is_some() {
        (style::WHITE, false)
    } else {
        (style::LABEL, false)
    };
    style::field(
        out,
        color,
        indent,
        "interp",
        interp,
        "Path of the loader. null means this file starts itself.",
        fg,
        bold,
    );
    let names = commands.load.names();
    write_needed(out, commands, &names, color, indent, names_only);
    claim::write_search(
        out,
        color,
        indent,
        "macho",
        &commands.load.runpath,
        &commands.load.rpath,
    );
    claim::write_delay(out, color, indent, "macho", &commands.load.delay);
    claim::write_dlopen(out, color, indent, "macho", &names);
    style::field(
        out,
        color,
        indent,
        "pie",
        if commands.load.pie { "true" } else { "false" },
        "MH_PIE bit in the header flags.",
        if commands.load.pie {
            style::GREEN
        } else {
            style::LABEL
        },
        commands.load.pie,
    );
}

fn write_needed(
    out: &mut String,
    commands: &Commands,
    names: &[String],
    color: bool,
    indent: usize,
    names_only: bool,
) {
    let rows = &commands.load.needed;
    let note = if names_only {
        claim::needed_note("macho")
    } else {
        NEEDED_NOTE
    };
    let fg = if rows.is_empty() {
        style::LABEL
    } else {
        style::WHITE
    };
    if names_only || rows.is_empty() {
        style::noted_list(
            out,
            color,
            indent,
            "needed",
            names,
            note,
            fg,
            false,
            crate::library::note,
        );
        return;
    }
    let gap = " ".repeat(indent);
    let painted = style::paint(color, "needed:", style::LABEL, false);
    let plain = indent + "needed:".len();
    style::align_note(out, color, &format!("{gap}{painted}"), plain, note);
    let item_gap = " ".repeat(indent + 2);
    let child_gap = " ".repeat(indent + 4);
    for (index, row) in rows.iter().enumerate() {
        let shown_plain = style::yaml_scalar(&row.name);
        let shown = style::file_text(color, &row.name);
        let dash = style::paint(color, "-", style::LABEL, false);
        let prefix = format!("{item_gap}{dash} {shown}");
        let plain_len = item_gap.len() + 2 + shown_plain.chars().count();
        match crate::library::note(&row.name) {
            Some(item) => style::align_note(out, color, &prefix, plain_len, item),
            None => {
                let _ = writeln!(out, "{prefix}");
            }
        }
        if let Some(word) = row.cmd.word() {
            let _ = writeln!(out, "{child_gap}cmd: {word}");
        }
        let _ = writeln!(out, "{child_gap}compat: {}", row.compat);
        let _ = writeln!(out, "{child_gap}current: {}", row.current);
        let used = commands.extra.uses.get(index + 1).copied().unwrap_or(0);
        let _ = writeln!(out, "{child_gap}used: {used}");
        if commands.extra.rows {
            write_used_names(out, &child_gap, &commands.extra, (index + 1) as u32);
        }
    }
}

fn write_used_names(out: &mut String, child_gap: &str, extra: &symbols::Extra, ordinal: u32) {
    let names = symbols::import_names(extra, ordinal);
    if names.is_empty() {
        return;
    }
    let _ = writeln!(out, "{child_gap}symbols:");
    let dash = format!("{child_gap}  ");
    for name in names {
        let _ = writeln!(out, "{dash}- {}", style::yaml_scalar(name));
    }
}

fn library_ordinal(ordinal: u32, needed: usize) -> bool {
    match usize::try_from(ordinal) {
        Ok(index) => index >= 1 && index <= needed,
        Err(_) => false,
    }
}

fn size_rung(out: &mut String, thin: &Thin, color: bool) {
    style::rung(
        out,
        color,
        "sizes",
        "Rung 2. Size of this header and size of the load commands.",
        style::BLUE,
        true,
    );
    style::field(
        out,
        color,
        2,
        "header_size",
        &thin.class.header_bytes().to_string(),
        "Bytes in this header. 28 or 32.",
        style::WHITE,
        false,
    );
    let (fg, bold) = style::number_tone(thin.ncmds as u64, style::WHITE, false);
    style::field(
        out,
        color,
        2,
        "ncmds",
        &thin.ncmds.to_string(),
        "4 bytes. Number of load commands.",
        fg,
        bold,
    );
    let (fg, bold) = style::number_tone(thin.sizeofcmds as u64, style::WHITE, false);
    style::field(
        out,
        color,
        2,
        "sizeofcmds",
        &thin.sizeofcmds.to_string(),
        "4 bytes. Byte size of all load commands.",
        fg,
        bold,
    );
}

fn where_rung(out: &mut String, thin: &Thin, commands: &Commands, color: bool) {
    let wide = if commands.entry_is_offset || thin.class != super::header::Class::Mach32 {
        "8 bytes."
    } else {
        "4 bytes."
    };
    let entry_note = if commands.entry_is_offset {
        format!("{wide} File offset of the entry point.")
    } else {
        format!("{wide} Assumed address of the first instruction.")
    };
    style::rung(
        out,
        color,
        "where",
        "Entry and load-command offset.",
        style::FG,
        true,
    );
    let (fg, bold) = style::number_tone(commands.entry, style::FG, true);
    style::field(
        out,
        color,
        2,
        "entry",
        &style::hex(commands.entry),
        &entry_note,
        fg,
        bold,
    );
    let entry_name = commands.extra.entry_name.as_deref().unwrap_or("null");
    let (fg, bold) = style::word_tone(commands.extra.entry_name.as_deref(), style::FG, true);
    style::field(
        out,
        color,
        2,
        "entry_name",
        entry_name,
        commands.extra.entry_note,
        fg,
        bold,
    );
    style::field(
        out,
        color,
        2,
        "cmdoff",
        &thin.class.header_bytes().to_string(),
        "File offset of the load commands.",
        style::WHITE,
        false,
    );
}

fn section_rung(out: &mut String, commands: &Commands, color: bool) {
    style::rung(
        out,
        color,
        "sections",
        "The loader does not read the symbol table to start the program.",
        style::LABEL,
        false,
    );
    let (fg, bold) = style::number_tone(commands.symoff as u64, style::WHITE, false);
    style::field(
        out,
        color,
        2,
        "symoff",
        &commands.symoff.to_string(),
        "4 bytes. File offset of the symbol table.",
        fg,
        bold,
    );
    style::field(
        out,
        color,
        2,
        "nsyms",
        &commands.nsyms.to_string(),
        "4 bytes. Number of symbols.",
        style::WHITE,
        false,
    );
    style::field(
        out,
        color,
        2,
        "stroff",
        &commands.stroff.to_string(),
        "4 bytes. File offset of the string table.",
        style::WHITE,
        false,
    );
    style::field(
        out,
        color,
        2,
        "strsize",
        &commands.strsize.to_string(),
        "4 bytes. Byte size of the string table.",
        style::WHITE,
        false,
    );
}

fn write_extra(out: &mut String, commands: &Commands, color: bool, indent: usize, entry: bool) {
    let extra = &commands.extra;
    let field = indent + 2;
    let mut saw = false;
    if let Some(build) = extra.build {
        group(
            out,
            color,
            indent,
            "build",
            &format!("[{}] Platform, minimum OS, and SDK.", build.source),
        );
        let name = platform_name(build.platform);
        let text = style::named(name, u64::from(build.platform));
        let (fg, bold) = style::word_tone(name, style::BLUE, true);
        style::field(
            out,
            color,
            field,
            "platform",
            &text,
            "Platform id.",
            fg,
            bold,
        );
        style::field(
            out,
            color,
            field,
            "minos",
            &symbols::dotted(build.minos),
            "Minimum OS version.",
            style::WHITE,
            false,
        );
        style::field(
            out,
            color,
            field,
            "sdk",
            &symbols::dotted(build.sdk),
            "SDK version.",
            style::WHITE,
            false,
        );
        saw = true;
    }
    if saw {
        style::blank(out);
    }
    group(out, color, indent, "symbols", "Loader symbol records.");
    let needed = commands.load.needed.len();
    if let Some(count) = extra.chained {
        let (fg, bold) = style::number_tone(u64::from(count), style::WHITE, false);
        style::field(
            out,
            color,
            field,
            "chained_fixups",
            &count.to_string(),
            "[LC_DYLD_CHAINED_FIXUPS] Imported symbols.",
            fg,
            bold,
        );
        if extra.rows {
            write_chained(out, color, field, extra, needed);
        }
    }
    if let Some(count) = extra.bind {
        let (fg, bold) = style::number_tone(u64::from(count), style::WHITE, false);
        style::field(
            out,
            color,
            field,
            "bind",
            &count.to_string(),
            "[LC_DYLD_INFO] Imported symbols.",
            fg,
            bold,
        );
        if extra.rows {
            write_bind(out, color, field, extra, needed);
        }
    }
    let export_note = format!("[{}] Exported symbols.", extra.export_cmd);
    if extra.rows {
        named_list(
            out,
            color,
            field,
            "exports",
            &export_note,
            &extra.export_names,
        );
    } else {
        let (fg, bold) = style::number_tone(u64::from(extra.exports), style::WHITE, false);
        style::field(
            out,
            color,
            field,
            "exports",
            &extra.exports.to_string(),
            &export_note,
            fg,
            bold,
        );
    }
    style::field(
        out,
        color,
        field,
        "symtab",
        if extra.symtab { "true" } else { "false" },
        "[LC_SYMTAB] The loader does not read this table.",
        if extra.symtab {
            style::WHITE
        } else {
            style::LABEL
        },
        false,
    );
    if entry {
        let shown = extra.entry_name.as_deref().unwrap_or("null");
        let (fg, bold) = style::word_tone(extra.entry_name.as_deref(), style::FG, true);
        style::field(
            out,
            color,
            field,
            "entry_name",
            shown,
            extra.entry_note,
            fg,
            bold,
        );
    }
}

fn write_chained(
    out: &mut String,
    color: bool,
    indent: usize,
    extra: &symbols::Extra,
    needed: usize,
) {
    let any = extra
        .chained_rows
        .iter()
        .any(|row| !library_ordinal(row.ordinal, needed));
    if !any {
        return;
    }
    key_note(
        out,
        color,
        indent,
        "other",
        "[LC_DYLD_CHAINED_FIXUPS] Imports with no needed library.",
    );
    let dash = " ".repeat(indent + 2);
    let sub = " ".repeat(indent + 4);
    for row in &extra.chained_rows {
        if library_ordinal(row.ordinal, needed) {
            continue;
        }
        let _ = writeln!(out, "{dash}- ordinal: {}", row.ordinal);
        let _ = writeln!(out, "{sub}name: {}", style::yaml_scalar(&row.name));
        let _ = writeln!(
            out,
            "{sub}weak: {}",
            if row.weak { "true" } else { "false" }
        );
    }
}

fn write_bind(out: &mut String, color: bool, indent: usize, extra: &symbols::Extra, needed: usize) {
    let any = extra
        .bind_rows
        .iter()
        .any(|row| !library_ordinal(row.ordinal, needed));
    if !any {
        return;
    }
    key_note(
        out,
        color,
        indent,
        "other",
        "[LC_DYLD_INFO] Imports with no needed library.",
    );
    let dash = " ".repeat(indent + 2);
    let sub = " ".repeat(indent + 4);
    for row in &extra.bind_rows {
        if library_ordinal(row.ordinal, needed) {
            continue;
        }
        let _ = writeln!(out, "{dash}- ordinal: {}", row.ordinal);
        let _ = writeln!(out, "{sub}name: {}", style::yaml_scalar(&row.name));
    }
}

fn named_list(
    out: &mut String,
    color: bool,
    indent: usize,
    key: &str,
    note: &str,
    names: &[String],
) {
    key_note(out, color, indent, key, note);
    let dash = " ".repeat(indent + 2);
    for name in names {
        let _ = writeln!(out, "{dash}- {}", style::yaml_scalar(name));
    }
}

fn group(out: &mut String, color: bool, indent: usize, title: &str, note: &str) {
    key_note(out, color, indent, title, note);
}

fn key_note(out: &mut String, color: bool, indent: usize, title: &str, note: &str) {
    let gap = " ".repeat(indent);
    let painted = style::paint(color, &format!("{title}:"), style::FG, true);
    let plain = indent + title.len() + 1;
    style::align_note(out, color, &format!("{gap}{painted}"), plain, note);
}

fn file_tone(kind: FileType) -> (style::Rgb, bool) {
    match kind {
        FileType::Object => (style::TEAL, false),
        FileType::Execute => (style::GREEN, true),
        FileType::Dylib | FileType::Bundle => (style::MAGENTA, true),
        FileType::Dylinker => (style::BLUE, true),
        FileType::Core => (style::MAGENTA, true),
        _ => (style::PINK, false),
    }
}

#[cfg(test)]
mod tests {
    use super::super::header::exec64;
    use super::super::load::{self, Commands, Kind, Load, Slice};
    use super::*;

    fn still() -> Commands {
        Commands {
            load: Load {
                kind: Kind::Static,
                interp: None,
                needed: Vec::new(),
                runpath: Vec::new(),
                rpath: Vec::new(),
                delay: Vec::new(),
                pie: false,
            },
            entry: 0x1000,
            entry_is_offset: true,
            symoff: 0,
            nsyms: 0,
            stroff: 0,
            strsize: 0,
            extra: load::Extra::empty(),
        }
    }

    #[test]
    fn notes_use_the_same_rungs() {
        let text = render("exec", &exec64(), &still(), &[], false);
        for key in ["type:", "load:", "sizes:", "where:", "sections:"] {
            assert!(text.lines().any(|line| line.starts_with(key)), "{text}");
        }
        assert!(text.contains("format: macho"), "{text}");
        assert!(text.contains("magic: 0xFEEDFACF"), "{text}");
        assert!(text.contains("filetype: EXECUTE"), "{text}");
        assert!(text.contains("cputype_name: AArch64 (16777228)"), "{text}");
        assert!(text.contains("cpusubtype_name: ARM64_ALL (0)"), "{text}");
        assert!(
            !text
                .lines()
                .any(|line| line.trim_start().starts_with("universal:")),
            "{text}"
        );
        assert!(text.contains("kind: static"), "{text}");
        assert!(text.contains("needed: []"), "{text}");
        assert!(
            text.contains("[none] Mach-O has no RUNPATH record."),
            "{text}"
        );
        assert!(
            text.contains("[LC_RPATH] Directories the loader uses for @rpath."),
            "{text}"
        );
        assert!(!text.contains("hermetic:"), "{text}");
        assert!(!text.contains("says:"), "{text}");
        assert!(text.contains("# MH_PIE bit in the header flags."), "{text}");
        assert!(
            text.contains("# 8 bytes. File offset of the entry point."),
            "{text}"
        );
        assert!(!text.contains('\u{1b}'), "{text}");
    }

    #[test]
    fn arm64_32_has_its_own_cpu_word() {
        use super::super::header::{Class, Data, FileType, Image, Thin};
        let image = Image::Thin(Thin {
            class: Class::Mach64,
            data: Data::Little,
            magic: 0xfeed_facf,
            cputype: 0x0200_000c,
            cpusubtype: 1,
            filetype: FileType::Dylib,
            ncmds: 0,
            sizeofcmds: 0,
            flags: 0,
        });
        let text = render("lib", &image, &still(), &[], false);
        assert!(text.contains("cputype_name: ARM64_32 (33554444)"), "{text}");
        assert!(text.contains("cpusubtype_name: ARM64_32_V8 (1)"), "{text}");
    }

    #[test]
    fn arm64e_x1_keeps_the_capability_bit_and_names_the_variant() {
        use super::super::header::{Class, Data, FileType, Image, Thin};
        // Siri.app slice 1. Raw value is 0x8000000C. Low 8 bits are 12.
        let image = Image::Thin(Thin {
            class: Class::Mach64,
            data: Data::Little,
            magic: 0xfeed_facf,
            cputype: 0x0100_000c,
            cpusubtype: 0x8000_000c,
            filetype: FileType::Execute,
            ncmds: 0,
            sizeofcmds: 0,
            flags: 0,
        });
        let text = render("Siri", &image, &still(), &[], false);
        assert!(
            text.contains("cpusubtype_name: ARM64E_X1 (2147483660)"),
            "{text}"
        );
        assert!(text.contains("cputype_name: AArch64 (16777228)"), "{text}");
    }

    #[test]
    fn arm64_x1_has_its_own_subtype_word() {
        use super::super::header::{Class, Data, FileType, Image, Thin};
        let image = Image::Thin(Thin {
            class: Class::Mach64,
            data: Data::Little,
            magic: 0xfeed_facf,
            cputype: 0x0100_000c,
            cpusubtype: 3,
            filetype: FileType::Execute,
            ncmds: 0,
            sizeofcmds: 0,
            flags: 0,
        });
        let text = render("bin", &image, &still(), &[], false);
        assert!(text.contains("cpusubtype_name: ARM64_X1 (3)"), "{text}");
    }

    #[test]
    fn needed_names_are_a_block_list_in_file_order() {
        let mut commands = still();
        commands.load.needed = vec![
            load::Dylib {
                cmd: load::DylibCmd::Load,
                name: "@rpath/libsherpa-onnx-c-api.dylib".into(),
                compat: "1.0.0".into(),
                current: "2.0.0".into(),
            },
            load::Dylib {
                cmd: load::DylibCmd::Weak,
                name: "/usr/lib/libSystem.B.dylib".into(),
                compat: "1.0.0".into(),
                current: "1351.0.0".into(),
            },
        ];
        let text = render("exec", &exec64(), &commands, &[], false);
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
            lines[at].contains("[LC_LOAD_DYLIB] One row is one load command. No cmd means load."),
            "{}",
            lines[at]
        );
        assert!(
            lines[at + 1].contains("@rpath/libsherpa-onnx-c-api.dylib"),
            "{}",
            lines[at + 1]
        );
        assert!(
            lines[at + 1].contains("A library from rpath."),
            "{}",
            lines[at + 1]
        );
        assert_eq!(lines[at + 2], "      compat: 1.0.0");
        assert_eq!(lines[at + 3], "      current: 2.0.0");
        assert_eq!(lines[at + 4], "      used: 0");
        assert!(
            lines[at + 5].contains("/usr/lib/libSystem.B.dylib"),
            "{}",
            lines[at + 5]
        );
        assert!(
            lines[at + 5].contains("libc, libm, and pthread."),
            "{}",
            lines[at + 5]
        );
        assert_eq!(
            lines[at + 5].find('#'),
            Some(style::COLUMN + 2),
            "{}",
            lines[at + 5]
        );
        assert_eq!(lines[at + 6], "      cmd: weak");
        assert_eq!(lines[at + 7], "      compat: 1.0.0");
        assert_eq!(lines[at + 8], "      current: 1351.0.0");
        assert_eq!(lines[at + 9], "      used: 0");
        assert!(!text.contains("dylib:"), "{text}");
        assert!(
            lines[at + 10].starts_with("  runpath: []"),
            "{}",
            lines[at + 10]
        );
        assert!(
            lines[at + 11].starts_with("  rpath: []"),
            "{}",
            lines[at + 11]
        );
        assert!(
            lines[at + 12].starts_with("  delay: []"),
            "{}",
            lines[at + 12]
        );
        assert!(
            lines[at + 13].starts_with("  dlopen: null"),
            "{}",
            lines[at + 13]
        );
        assert!(lines[at + 14].starts_with("  pie:"), "{}", lines[at + 14]);
    }

    #[test]
    fn a_needed_row_lists_the_symbols_that_library_supplies() {
        let mut commands = still();
        commands.load.needed = vec![
            load::link("/usr/lib/libSystem.B.dylib"),
            load::link("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation"),
        ];
        commands.extra.rows = true;
        commands.extra.chained = Some(4);
        commands.extra.uses = vec![1, 2, 1];
        commands.extra.chained_rows = vec![
            symbols::ChainedImport {
                ordinal: 1,
                name: "_malloc".into(),
                weak: false,
            },
            symbols::ChainedImport {
                ordinal: 1,
                name: "_free".into(),
                weak: false,
            },
            symbols::ChainedImport {
                ordinal: 2,
                name: "_CFRelease".into(),
                weak: false,
            },
            symbols::ChainedImport {
                ordinal: 0,
                name: "_dyld_stub".into(),
                weak: false,
            },
        ];
        let text = render("exec", &exec64(), &commands, &[], false);
        let lines: Vec<&str> = text.lines().collect();
        let system = lines
            .iter()
            .position(|line| line.contains("libSystem.B.dylib"))
            .expect("libSystem");
        assert_eq!(lines[system + 3], "      used: 2");
        assert_eq!(lines[system + 4], "      symbols:");
        assert_eq!(lines[system + 5], "        - _malloc");
        assert_eq!(lines[system + 6], "        - _free");
        let foundation = lines
            .iter()
            .position(|line| line.contains("CoreFoundation"))
            .expect("CoreFoundation");
        assert!(foundation > system, "{text}");
        assert_eq!(lines[foundation + 3], "      used: 1");
        assert_eq!(lines[foundation + 4], "      symbols:");
        assert_eq!(lines[foundation + 5], "        - _CFRelease");
        assert_eq!(text.matches("_malloc").count(), 1, "{text}");
        assert!(text.contains("ordinal: 0"), "{text}");
        assert!(text.contains("_dyld_stub"), "{text}");
        assert!(!text.contains("ordinal: 1"), "{text}");
        assert!(!text.contains("ordinal: 2"), "{text}");
    }

    #[test]
    fn a_needed_row_counts_uses_when_symbol_names_are_off() {
        let mut commands = still();
        commands.load.needed = vec![load::link("/usr/lib/libSystem.B.dylib")];
        commands.extra.chained = Some(2);
        commands.extra.uses = vec![0, 2];
        let text = render("exec", &exec64(), &commands, &[], false);
        assert!(text.contains("used: 2"), "{text}");
        assert!(text.contains("chained_fixups: 2"), "{text}");
        assert!(!text.lines().any(|line| line == "      symbols:"), "{text}");
    }

    #[test]
    fn rpath_names_are_a_block_list_in_file_order() {
        let mut commands = still();
        commands.load.rpath = vec!["@executable_path/../lib".into(), "/opt/lib".into()];
        let text = render("exec", &exec64(), &commands, &[], false);
        let lines: Vec<&str> = text.lines().collect();
        let at = lines
            .iter()
            .position(|line| line.starts_with("  runpath:"))
            .unwrap();
        assert!(lines[at].starts_with("  runpath: []"), "{}", lines[at]);
        assert!(
            lines[at].contains("[none] Mach-O has no RUNPATH record."),
            "{}",
            lines[at]
        );
        assert!(
            lines[at + 1].contains("[LC_RPATH] Directories the loader uses for @rpath."),
            "{}",
            lines[at + 1]
        );
        assert_eq!(lines[at + 2], "    - @executable_path/../lib");
        assert_eq!(lines[at + 3], "    - /opt/lib");
        assert!(
            lines[at + 4].starts_with("  delay: []"),
            "{}",
            lines[at + 4]
        );
        assert!(
            lines[at + 5].contains("[none] libSystem is common. That name does not prove dlopen."),
            "{}",
            lines[at + 5]
        );
        assert!(lines[at + 6].starts_with("  pie:"), "{}", lines[at + 6]);
    }

    fn pie_slice(cpu: u32, sub: u32, offset: u64, needed: &str, rpath: &str) -> Slice {
        use super::super::header::{Class, Data, FileType, Thin};
        Slice {
            cputype: cpu,
            cpusubtype: sub,
            offset,
            size: 64,
            thin: Some(Thin {
                class: Class::Mach64,
                data: Data::Little,
                magic: 0xfeed_facf,
                cputype: cpu,
                cpusubtype: sub,
                filetype: FileType::Execute,
                ncmds: 3,
                sizeofcmds: 0,
                flags: 0x200000,
            }),
            commands: Commands {
                load: Load {
                    kind: Kind::DynamicPie,
                    interp: Some("/usr/lib/dyld".into()),
                    needed: vec![load::link(needed)],
                    runpath: Vec::new(),
                    rpath: vec![rpath.into()],
                    delay: Vec::new(),
                    pie: true,
                },
                entry: 0,
                entry_is_offset: false,
                symoff: 0,
                nsyms: 0,
                stroff: 0,
                strsize: 0,
                extra: load::Extra::empty(),
            },
        }
    }

    fn fat_image() -> Image {
        use super::super::header::{Arch, Class, Data, Fat};
        Image::Fat(Fat {
            class: Class::Fat,
            data: Data::Big,
            magic: 0xcafe_babe,
            nfat_arch: 2,
            arches: vec![
                Arch {
                    cputype: 0x0100_0007,
                    cpusubtype: 3,
                    offset: 0x4000,
                    size: 64,
                },
                Arch {
                    cputype: 0x0100_000c,
                    cpusubtype: 0x8000_0002,
                    offset: 0x5000,
                    size: 64,
                },
            ],
        })
    }

    #[test]
    fn fat_page_opens_each_slice_and_keeps_a_colon_in_rpath() {
        let slices = vec![
            pie_slice(0x0100_0007, 3, 0x4000, "libx.dylib", "@executable_path/x:y"),
            pie_slice(
                0x0100_000c,
                0x8000_0002,
                0x5000,
                "liba.dylib",
                "@executable_path/a",
            ),
        ];
        let commands = self::load::commands_for(&slices);
        let text = render("Photo", &fat_image(), &commands, &slices, false);
        assert!(text.contains("magic: 0xCAFEBABE"), "{text}");
        assert!(text.contains("class: FAT"), "{text}");
        assert!(text.contains("universal: universal2"), "{text}");
        assert!(!text.contains("This page does not open a slice."), "{text}");
        assert!(
            text.contains("These slices do not share one load. Read each slice."),
            "{text}"
        );
        assert!(
            !text.lines().any(|line| line.starts_with("load:")),
            "{text}"
        );
        assert!(text.contains("cputype_name: x86-64 (16777223)"), "{text}");
        assert!(text.contains("cpusubtype_name: X86_64_ALL (3)"), "{text}");
        assert!(text.contains("cputype_name: AArch64 (16777228)"), "{text}");
        assert!(
            text.contains("cpusubtype_name: ARM64E (2147483650)"),
            "{text}"
        );
        assert!(text.contains("offset: 0x4000"), "{text}");
        assert!(text.contains("offset: 0x5000"), "{text}");
        let x = text.find("libx.dylib").expect("libx");
        let a = text.find("liba.dylib").expect("liba");
        assert!(x < a, "{text}");
        assert!(text.contains("@executable_path/x:y"), "{text}");
        assert!(!text.contains("@executable_path/x\n"), "{text}");
    }

    fn universal_value(text: &str) -> Option<&str> {
        text.lines().find_map(|line| {
            let line = line.trim_start();
            let rest = line.strip_prefix("universal:")?;
            Some(rest.split_whitespace().next().unwrap_or(""))
        })
    }

    #[test]
    fn fat_pages_name_universal_and_universal2() {
        let pair = vec![
            pie_slice(0x0100_0007, 3, 0x4000, "liba.dylib", "@executable_path/a"),
            pie_slice(0x0100_000c, 2, 0x5000, "liba.dylib", "@executable_path/a"),
        ];
        let pair_text = render("pair", &fat_image(), &still(), &pair, false);
        assert_eq!(universal_value(&pair_text), Some("universal2"));

        let siri = vec![
            pie_slice(0x0100_000c, 2, 0x4000, "liba.dylib", "@executable_path/a"),
            pie_slice(0x0100_000c, 12, 0x5000, "liba.dylib", "@executable_path/a"),
        ];
        let siri_text = render("siri", &fat_image(), &still(), &siri, false);
        assert_eq!(universal_value(&siri_text), Some("universal"));

        let watch = vec![
            pie_slice(0x0100_0007, 3, 0x4000, "liba.dylib", "@executable_path/a"),
            pie_slice(0x0200_000c, 1, 0x5000, "liba.dylib", "@executable_path/a"),
        ];
        let watch_text = render("watch", &fat_image(), &still(), &watch, false);
        assert_eq!(universal_value(&watch_text), Some("universal"));

        let old = vec![
            pie_slice(7, 0, 0x4000, "liba.dylib", "@executable_path/a"),
            pie_slice(0x0100_000c, 0, 0x5000, "liba.dylib", "@executable_path/a"),
        ];
        let old_text = render("old", &fat_image(), &still(), &old, false);
        assert_eq!(universal_value(&old_text), Some("universal"));

        let empty = render("empty", &fat_image(), &still(), &[], false);
        assert_eq!(universal_value(&empty), Some("null"));
    }

    #[test]
    fn matching_slices_repeat_one_load() {
        let slices = vec![
            pie_slice(
                0x0100_0007,
                3,
                0x4000,
                "liba.dylib",
                "@executable_path/../Frameworks",
            ),
            pie_slice(
                0x0100_000c,
                0,
                0x5000,
                "liba.dylib",
                "@executable_path/../Frameworks",
            ),
        ];
        let commands = self::load::commands_for(&slices);
        let text = render("Photo", &fat_image(), &commands, &slices, false);
        assert!(text.contains("Every slice has these load facts."), "{text}");
        assert!(text.lines().any(|line| line.starts_with("load:")), "{text}");
        assert!(text.contains("      - liba.dylib"), "{text}");
        assert!(text.contains("    - liba.dylib"), "{text}");
        assert!(text.contains("@executable_path/../Frameworks"), "{text}");
    }

    #[test]
    fn a_closed_slice_says_it_does_not_open() {
        let mut slice = pie_slice(0x0100_0007, 3, 0x4000, "liba.dylib", "/opt/lib");
        slice.thin = None;
        slice.commands = still();
        slice.commands.load.kind = Kind::Other;
        let commands = load::commands_for(std::slice::from_ref(&slice));
        let text = render("Photo", &fat_image(), &commands, &[slice], false);
        assert!(text.contains("filetype: null"), "{text}");
        assert!(text.contains("This slice does not open."), "{text}");
        assert!(!text.contains("says:"), "{text}");
    }
}
