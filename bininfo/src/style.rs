/*
---
why: This file writes one page line and one JSON token.
uses: bininfo
how: Each page file calls this file to align a field and a note.
A long value keeps its note on the same line.
A file list paints the directory gray and the library name white.
A note is a light blue.
---
*/

//! Page lines and JSON tokens. No file format lives here.

use crate::kind::Kind;
use std::fmt::Write;

pub const COLUMN: usize = 32;

#[derive(Clone, Copy)]
pub struct Rgb(pub u8, pub u8, pub u8);

pub const FG: Rgb = Rgb(0xf2, 0xf4, 0xf8);
pub const NOTE: Rgb = Rgb(0x9d, 0xc8, 0xe6);
pub const LABEL: Rgb = Rgb(0x7b, 0x7c, 0x7e);
pub const GREEN: Rgb = Rgb(0x25, 0xbe, 0x6a);
pub const TEAL: Rgb = Rgb(0x08, 0xbd, 0xba);
pub const BLUE: Rgb = Rgb(0x78, 0xa9, 0xff);
pub const CYAN: Rgb = Rgb(0x33, 0xb1, 0xff);
pub const MAGENTA: Rgb = Rgb(0xbe, 0x95, 0xff);
pub const PINK: Rgb = Rgb(0xff, 0x7e, 0xb6);
pub const WHITE: Rgb = Rgb(0xdf, 0xdf, 0xe0);

pub fn blank(out: &mut String) {
    out.push('\n');
}

pub fn rung(out: &mut String, color: bool, title: &str, note: &str, fg: Rgb, bold: bool) {
    let text = format!("{title}:");
    let plain = text.chars().count();
    let title = paint(color, &text, fg, bold);
    align_note(out, color, &title, plain, note);
}

pub fn field(
    out: &mut String,
    color: bool,
    indent: usize,
    key: &str,
    value: &str,
    note: &str,
    fg: Rgb,
    bold: bool,
) {
    let shown = show(value);
    raw_field(out, color, indent, key, &shown, note, fg, bold);
}

pub fn raw_field(
    out: &mut String,
    color: bool,
    indent: usize,
    key: &str,
    shown: &str,
    note: &str,
    fg: Rgb,
    bold: bool,
) {
    emit(
        out,
        color,
        indent,
        key,
        shown,
        shown.chars().count(),
        note,
        fg,
        bold,
    );
}

/// Header hex. Digits are `A` through `F`, as in `0xCAFEBABE`.
pub fn hex(value: u64) -> String {
    format!("0x{value:X}")
}

pub fn yaml_list(names: &[String]) -> String {
    if names.is_empty() {
        return "[]".to_string();
    }
    let items: Vec<String> = names.iter().map(|name| show(name)).collect();
    format!("[{}]", items.join(", "))
}

// The note stays on the key line. A long flow value would push it down.
pub fn list_field(
    out: &mut String,
    color: bool,
    indent: usize,
    key: &str,
    names: &[String],
    note: &str,
    fg: Rgb,
    bold: bool,
) {
    write_list(out, color, indent, key, names, note, fg, bold, None, false);
}

pub fn path_list(
    out: &mut String,
    color: bool,
    indent: usize,
    key: &str,
    names: &[String],
    note: &str,
    fg: Rgb,
    bold: bool,
) {
    write_list(out, color, indent, key, names, note, fg, bold, None, true);
}

pub fn noted_list(
    out: &mut String,
    color: bool,
    indent: usize,
    key: &str,
    names: &[String],
    note: &str,
    fg: Rgb,
    bold: bool,
    note_of: fn(&str) -> Option<&'static str>,
) {
    write_list(
        out,
        color,
        indent,
        key,
        names,
        note,
        fg,
        bold,
        Some(note_of),
        true,
    );
}

fn write_list(
    out: &mut String,
    color: bool,
    indent: usize,
    key: &str,
    names: &[String],
    note: &str,
    fg: Rgb,
    bold: bool,
    note_of: Option<fn(&str) -> Option<&'static str>>,
    paths: bool,
) {
    if names.is_empty() {
        raw_field(out, color, indent, key, "[]", note, fg, bold);
        return;
    }
    let gap = " ".repeat(indent);
    let painted = paint(color, &format!("{key}:"), LABEL, false);
    let plain = indent + key.len() + 1;
    align_note(out, color, &format!("{gap}{painted}"), plain, note);
    let item_gap = " ".repeat(indent + 2);
    for name in names {
        let shown_plain = yaml_scalar(name);
        let shown = if paths {
            file_text(color, name)
        } else {
            paint(color, &shown_plain, fg, bold)
        };
        let dash = if paths {
            paint(color, "-", LABEL, false)
        } else {
            "-".to_string()
        };
        let prefix = format!("{item_gap}{dash} {shown}");
        match note_of.and_then(|note_of| note_of(name)) {
            Some(item) => {
                let plain = item_gap.len() + 2 + shown_plain.len();
                align_note(out, color, &prefix, plain, item);
            }
            None => {
                let _ = writeln!(out, "{prefix}");
            }
        }
    }
}

/// The directory is gray. The library name is white and bold.
pub fn paint_path(on: bool, text: &str) -> String {
    if !on {
        return text.to_string();
    }
    paint_parts(text, false)
}

pub(crate) fn file_text(color: bool, name: &str) -> String {
    let plain = yaml_scalar(name);
    if !color {
        return plain;
    }
    if plain == name {
        return paint_path(true, name);
    }
    let mut out = paint(true, "\"", LABEL, false);
    out.push_str(&paint_parts(name, true));
    out.push_str(&paint(true, "\"", LABEL, false));
    out
}

fn paint_parts(text: &str, escape: bool) -> String {
    let (start, end) = library_range(text);
    let mut out = String::new();
    if start > 0 {
        out.push_str(&paint_piece(&text[..start], LABEL, false, escape));
    }
    if start < end {
        out.push_str(&paint_piece(&text[start..end], FG, true, escape));
    }
    if end < text.len() {
        out.push_str(&paint_piece(&text[end..], LABEL, false, escape));
    }
    out
}

/// Byte range of the library name. A `.framework` component wins. Otherwise the last component wins.
fn library_range(text: &str) -> (usize, usize) {
    let bytes = text.as_bytes();
    let mut last = None;
    let mut framework = None;
    let mut index = 0;
    while index < bytes.len() {
        if bytes[index] == b'/' || bytes[index] == b'\\' {
            index += 1;
            continue;
        }
        let start = index;
        while index < bytes.len() && bytes[index] != b'/' && bytes[index] != b'\\' {
            index += 1;
        }
        last = Some((start, index));
        if text[start..index].ends_with(".framework") {
            framework = Some((start, index));
        }
    }
    framework.or(last).unwrap_or((0, 0))
}

fn paint_piece(text: &str, color: Rgb, bold: bool, escape: bool) -> String {
    if !escape {
        return paint(true, text, color, bold);
    }
    let mut shown = String::new();
    for c in text.chars() {
        match c {
            '\\' => shown.push_str("\\\\"),
            '"' => shown.push_str("\\\""),
            '\n' => shown.push_str("\\n"),
            c => shown.push(c),
        }
    }
    paint(true, &shown, color, bold)
}

pub fn kind_tone(kind: Kind) -> (Rgb, bool) {
    match kind {
        Kind::Static => (GREEN, true),
        Kind::DynamicPie | Kind::StaticPie => (CYAN, true),
        Kind::Dynamic => (BLUE, true),
        Kind::Shared => (MAGENTA, true),
        Kind::Rel => (TEAL, false),
        Kind::Other => (PINK, false),
    }
}

pub fn number_tone(n: u64, fg: Rgb, bold: bool) -> (Rgb, bool) {
    if n == 0 {
        (LABEL, false)
    } else {
        (fg, bold)
    }
}

pub fn word_tone(name: Option<&str>, fg: Rgb, bold: bool) -> (Rgb, bool) {
    if name.is_none() {
        (LABEL, false)
    } else {
        (fg, bold)
    }
}

// None stays "null". Do not append the raw number.
pub fn named(word: Option<&str>, raw: u64) -> String {
    match word {
        Some(word) => format!("{word} ({raw})"),
        None => "null".to_string(),
    }
}

/// `word (raw)` when the word is known. The raw number when it is not.
pub fn tagged(word: Option<&str>, raw: u64) -> String {
    match word {
        Some(word) => format!("{word} ({raw})"),
        None => raw.to_string(),
    }
}

pub fn paint(on: bool, text: &str, color: Rgb, bold: bool) -> String {
    if !on {
        return text.to_string();
    }
    let Rgb(r, g, b) = color;
    if bold {
        format!("\x1b[1;38;2;{r};{g};{b}m{text}\x1b[0m")
    } else {
        format!("\x1b[38;2;{r};{g};{b}m{text}\x1b[0m")
    }
}

fn emit(
    out: &mut String,
    color: bool,
    indent: usize,
    key: &str,
    shown: &str,
    width: usize,
    note: &str,
    fg: Rgb,
    bold: bool,
) {
    let plain = indent + key.len() + 2 + width;
    let gap = " ".repeat(indent);
    let key = paint(color, &format!("{key}:"), LABEL, false);
    let value = paint(color, shown, fg, bold);
    align_note(out, color, &format!("{gap}{key} {value}"), plain, note);
}

pub(crate) fn align_note(out: &mut String, color: bool, prefix: &str, plain: usize, note: &str) {
    let note = paint(color, &format!("# {note}"), NOTE, false);
    let gap = if plain <= COLUMN {
        COLUMN - plain + 2
    } else {
        2
    };
    let pad = " ".repeat(gap);
    let _ = writeln!(out, "{prefix}{pad}{note}");
}

pub(crate) fn yaml_scalar(value: &str) -> String {
    if !value.chars().any(|c| matches!(c, ':' | '#' | '\n' | '"')) {
        return value.to_string();
    }
    let mut out = String::with_capacity(value.len() + 2);
    out.push('"');
    for c in value.chars() {
        match c {
            '\\' => out.push_str("\\\\"),
            '"' => out.push_str("\\\""),
            '\n' => out.push_str("\\n"),
            c => out.push(c),
        }
    }
    out.push('"');
    out
}

fn show(value: &str) -> String {
    if value.contains('#') || value.contains('\n') || value.contains('"') {
        let escaped = value.replace('\\', "\\\\").replace('"', "\\\"");
        format!("\"{escaped}\"")
    } else {
        value.to_string()
    }
}

/// One JSON object. The caller chooses the keys.
pub struct Json {
    out: String,
}

impl Json {
    pub fn new() -> Self {
        Self {
            out: String::from("{"),
        }
    }

    pub fn str(&mut self, key: &str, value: &str) {
        self.comma();
        self.out.push_str(&json_str(key));
        self.out.push(':');
        self.out.push_str(&json_str(value));
    }

    pub fn opt(&mut self, key: &str, value: Option<&str>) {
        self.comma();
        self.out.push_str(&json_str(key));
        self.out.push(':');
        match value {
            Some(text) => self.out.push_str(&json_str(text)),
            None => self.out.push_str("null"),
        }
    }

    pub fn u64(&mut self, key: &str, value: u64) {
        self.comma();
        self.out.push_str(&json_str(key));
        self.out.push(':');
        self.out.push_str(&value.to_string());
    }

    pub fn u8s(&mut self, key: &str, bytes: &[u8]) {
        self.comma();
        self.out.push_str(&json_str(key));
        self.out.push_str(":[");
        for (i, byte) in bytes.iter().enumerate() {
            if i > 0 {
                self.out.push(',');
            }
            self.out.push_str(&byte.to_string());
        }
        self.out.push(']');
    }

    pub fn raw_array(&mut self, key: &str, values: &[String]) {
        self.comma();
        self.out.push_str(&json_str(key));
        self.out.push_str(":[");
        for (index, value) in values.iter().enumerate() {
            if index > 0 {
                self.out.push(',');
            }
            self.out.push_str(value);
        }
        self.out.push(']');
    }

    pub fn strings(&mut self, key: &str, values: &[String]) {
        self.comma();
        self.out.push_str(&json_str(key));
        self.out.push_str(":[");
        for (i, value) in values.iter().enumerate() {
            if i > 0 {
                self.out.push(',');
            }
            self.out.push_str(&json_str(value));
        }
        self.out.push(']');
    }

    pub fn raw(&mut self, key: &str, value: &str) {
        self.comma();
        self.out.push_str(&json_str(key));
        self.out.push(':');
        self.out.push_str(value);
    }

    pub fn flag(&mut self, key: &str, value: bool) {
        self.comma();
        self.out.push_str(&json_str(key));
        self.out.push(':');
        self.out.push_str(if value { "true" } else { "false" });
    }

    pub fn pairs(&mut self, key: &str, left: &str, right: &str, rows: &[(&str, &str)]) {
        self.comma();
        self.out.push_str(&json_str(key));
        self.out.push_str(":[");
        for (index, (a, b)) in rows.iter().enumerate() {
            if index > 0 {
                self.out.push(',');
            }
            self.out.push('{');
            self.out.push_str(&json_str(left));
            self.out.push(':');
            self.out.push_str(&json_str(a));
            self.out.push(',');
            self.out.push_str(&json_str(right));
            self.out.push(':');
            self.out.push_str(&json_str(b));
            self.out.push('}');
        }
        self.out.push(']');
    }

    pub fn load(
        &mut self,
        kind: &str,
        interp: Option<&str>,
        needed: &[String],
        runpath: &[String],
        rpath: &[String],
        delay: &[String],
        dlopen: Option<bool>,
        pie: bool,
    ) {
        self.comma();
        self.out.push_str("\"load\":{\"kind\":");
        self.out.push_str(&json_str(kind));
        self.out.push_str(",\"interp\":");
        match interp {
            Some(path) => self.out.push_str(&json_str(path)),
            None => self.out.push_str("null"),
        }
        self.out.push_str(",\"needed\":[");
        append_strings(&mut self.out, needed);
        self.out.push_str("],\"runpath\":[");
        append_strings(&mut self.out, runpath);
        self.out.push_str("],\"rpath\":[");
        append_strings(&mut self.out, rpath);
        self.out.push_str("],\"delay\":[");
        append_strings(&mut self.out, delay);
        self.out.push_str("],\"dlopen\":");
        match dlopen {
            Some(true) => self.out.push_str("true"),
            Some(false) => self.out.push_str("false"),
            None => self.out.push_str("null"),
        }
        self.out.push_str(",\"pie\":");
        self.out.push_str(if pie { "true" } else { "false" });
        self.out.push('}');
    }

    pub fn finish_inline(mut self) -> String {
        self.out.push('}');
        self.out
    }

    pub fn finish(mut self) -> String {
        self.out.push('}');
        self.out.push('\n');
        self.out
    }

    fn comma(&mut self) {
        if self.out.len() > 1 {
            self.out.push(',');
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn hex_uses_uppercase_digits() {
        assert_eq!(hex(0xcafe_babe), "0xCAFEBABE");
        assert_eq!(hex(0xfeed_facf), "0xFEEDFACF");
        assert_eq!(hex(0), "0x0");
        assert_eq!(hex(0x1000), "0x1000");
    }

    #[test]
    fn notes_share_one_column() {
        let mut out = String::new();
        align_note(&mut out, false, "type:", "type:".chars().count(), "Rung 1.");
        let class = "  class: ELF64";
        align_note(&mut out, false, class, class.chars().count(), "1 byte.");
        let columns: Vec<usize> = out.lines().map(|line| line.find('#').unwrap()).collect();
        assert_eq!(columns, vec![COLUMN + 2, COLUMN + 2]);
    }

    #[test]
    fn a_long_value_keeps_the_note_on_its_line() {
        let mut out = String::new();
        let value = "x".repeat(40);
        let prefix = format!("  e_ident: {value}");
        align_note(
            &mut out,
            false,
            &prefix,
            prefix.chars().count(),
            "16 bytes.",
        );
        let line = out.lines().next().unwrap();
        assert_eq!(out.lines().count(), 1, "{out}");
        assert!(line.contains(&value), "{line}");
        assert!(line.ends_with("  # 16 bytes."), "{line}");
    }

    #[test]
    fn an_empty_list_stays_on_one_line() {
        let mut out = String::new();
        list_field(
            &mut out,
            false,
            2,
            "needed",
            &[],
            "Library names the loader opens.",
            LABEL,
            false,
        );
        let line = out.lines().next().unwrap();
        assert!(line.contains("needed: []"), "{out}");
        assert_eq!(line.find('#'), Some(COLUMN + 2), "{line}");
        assert_eq!(out.lines().count(), 1, "{out}");
    }

    #[test]
    fn a_list_of_names_is_a_block_in_vec_order() {
        let names = vec![
            "@rpath/libsherpa-onnx-c-api.dylib".into(),
            "/usr/lib/libSystem.B.dylib".into(),
        ];
        let mut out = String::new();
        list_field(
            &mut out,
            false,
            2,
            "needed",
            &names,
            "Library names the loader opens.",
            WHITE,
            false,
        );
        let lines: Vec<&str> = out.lines().collect();
        assert_eq!(lines.len(), 3, "{out}");
        assert!(lines[0].starts_with("  needed:"), "{out}");
        assert!(!lines[0].contains('['), "{out}");
        assert_eq!(lines[0].find('#'), Some(COLUMN + 2), "{}", lines[0]);
        assert_eq!(lines[1], "    - @rpath/libsherpa-onnx-c-api.dylib");
        assert_eq!(lines[2], "    - /usr/lib/libSystem.B.dylib");
    }

    #[test]
    fn a_list_quotes_only_a_broken_scalar() {
        let names = vec![
            "a:b".into(),
            "a#b".into(),
            "a\"b".into(),
            "a\nb".into(),
            "a\\b".into(),
            "@rpath/ok.dylib".into(),
        ];
        let mut out = String::new();
        list_field(
            &mut out,
            false,
            2,
            "needed",
            &names,
            "Library names the loader opens.",
            WHITE,
            false,
        );
        let lines: Vec<&str> = out.lines().collect();
        assert_eq!(lines[1], "    - \"a:b\"");
        assert_eq!(lines[2], "    - \"a#b\"");
        assert_eq!(lines[3], "    - \"a\\\"b\"");
        assert_eq!(lines[4], "    - \"a\\nb\"");
        assert_eq!(lines[5], "    - a\\b");
        assert_eq!(lines[6], "    - @rpath/ok.dylib");
    }

    #[test]
    fn a_file_list_paints_the_library_name_white() {
        let names = vec![
            "/usr/lib/libSystem.B.dylib".into(),
            "@rpath/libtiny.dylib".into(),
            "@loader_path/../lib".into(),
            "/System/Library/Frameworks/AppKit.framework/Versions/C/AppKit".into(),
            "@executable_path/../Frameworks/AgUI.framework/Versions/A/AgUI".into(),
            r"C:\Windows\System32\kernel32.dll".into(),
            "a:b/c".into(),
        ];
        let mut out = String::new();
        path_list(
            &mut out,
            true,
            2,
            "needed",
            &names,
            "Library names the loader opens.",
            LABEL,
            false,
        );
        let gray = "\u{1b}[38;2;123;124;126m";
        let file = "\u{1b}[1;38;2;242;244;248m";
        assert!(
            out.contains(&format!("{gray}/usr/lib/\u{1b}[0m{file}libSystem.B.dylib")),
            "{out}"
        );
        assert!(
            out.contains(&format!("{gray}@rpath/\u{1b}[0m{file}libtiny.dylib")),
            "{out}"
        );
        assert!(
            out.contains(&format!("{gray}@loader_path/../\u{1b}[0m{file}lib")),
            "{out}"
        );
        assert!(
            out.contains(&format!(
                "{gray}/System/Library/Frameworks/\u{1b}[0m{file}AppKit.framework\u{1b}[0m{gray}/Versions/C/AppKit"
            )),
            "{out}"
        );
        assert!(
            out.contains(&format!(
                "{gray}@executable_path/../Frameworks/\u{1b}[0m{file}AgUI.framework\u{1b}[0m{gray}/Versions/A/AgUI"
            )),
            "{out}"
        );
        assert!(!out.contains("\u{1b}[1;38;2;190;149;255m"), "{out}");
        assert!(
            out.contains(&format!(
                "{gray}\"\u{1b}[0m{gray}C:\\\\Windows\\\\System32\\\\\u{1b}[0m{file}kernel32.dll\u{1b}[0m{gray}\""
            )),
            "{out}"
        );
        assert!(
            out.contains(&format!(
                "{gray}\"\u{1b}[0m{gray}a:b/\u{1b}[0m{file}c\u{1b}[0m{gray}\""
            )),
            "{out}"
        );
        let visible: String = strip_color(out.lines().nth(1).unwrap());
        assert_eq!(visible, "    - /usr/lib/libSystem.B.dylib");
        let key = strip_color(out.lines().next().unwrap());
        assert_eq!(key.find('#'), Some(COLUMN + 2), "{key}");

        let mut plain = String::new();
        path_list(
            &mut plain,
            false,
            2,
            "needed",
            &names[..1],
            "Library names the loader opens.",
            WHITE,
            false,
        );
        assert!(
            plain.contains("    - /usr/lib/libSystem.B.dylib"),
            "{plain}"
        );

        let mut property = String::new();
        list_field(
            &mut property,
            true,
            2,
            "property",
            &["x86-64-v2".to_string()],
            "[.note.gnu.property]",
            WHITE,
            false,
        );
        assert!(
            property.contains("\u{1b}[38;2;223;223;224mx86-64-v2"),
            "{property}"
        );
        assert!(!property.contains(file), "{property}");
    }

    fn strip_color(text: &str) -> String {
        let mut out = String::new();
        let mut chars = text.chars();
        while let Some(c) = chars.next() {
            if c == '\u{1b}' {
                for n in chars.by_ref() {
                    if n == 'm' {
                        break;
                    }
                }
                continue;
            }
            out.push(c);
        }
        out
    }
}

fn append_strings(out: &mut String, items: &[String]) {
    for (index, name) in items.iter().enumerate() {
        if index > 0 {
            out.push(',');
        }
        out.push_str(&json_str(name));
    }
}

fn json_str(text: &str) -> String {
    let mut out = String::with_capacity(text.len() + 2);
    out.push('"');
    for c in text.chars() {
        match c {
            '"' => out.push_str("\\\""),
            '\\' => out.push_str("\\\\"),
            '\n' => out.push_str("\\n"),
            '\r' => out.push_str("\\r"),
            '\t' => out.push_str("\\t"),
            c if (c as u32) < 0x20 => out.push_str(&format!("\\u{:04x}", c as u32)),
            c => out.push(c),
        }
    }
    out.push('"');
    out
}
