/*
---
why: This file writes one PE header as one JSON object.
uses: bininfo
how: bininfo calls this file after it reads the header.
This file does not read PE bytes.
---
*/

//! One JSON object for one PE header.
//! This file does not read PE bytes.

use super::header::Header;
use super::load::Load;
use super::symbols::{self, Import, Symbols};
use super::words::{class_name, machine_name, magic_name, subsystem_name};
use crate::claim;
use crate::style::Json;

#[cfg(test)]
pub fn render(file: &str, header: &Header, load: &Load) -> String {
    render_with(file, header, load, &Symbols::empty())
}

pub fn render_with(file: &str, header: &Header, load: &Load, symbols: &Symbols) -> String {
    let mut out = Json::new();
    out.str("file", file);
    out.str("format", "pe");
    if header.dos == 0 {
        out.opt("magic", None);
    } else {
        out.str("magic", magic_name(header.dos));
    }
    out.opt(
        "signature",
        if header.class == super::header::Class::Coff {
            None
        } else {
            Some("PE")
        },
    );
    out.str("class", class_name(header.class));
    out.str("data", "little-endian");
    out.u64("machine", header.machine as u64);
    out.opt("machine_name", machine_name(header.machine));
    out.u64("characteristics", header.characteristics as u64);
    out.u64("subsystem", header.subsystem as u64);
    out.opt("subsystem_name", subsystem_name(header.subsystem));
    if let Some(os) = symbols.os {
        out.str("os_version", &symbols::pair_text(os));
    }
    if let Some(pair) = symbols.subsystem {
        out.str("subsystem_version", &symbols::pair_text(pair));
    }
    out.u64("dll_characteristics", header.dll_characteristics as u64);
    out.u64("size_of_optional_header", header.opt_size as u64);
    out.u64("number_of_sections", header.sections as u64);
    out.u64("size_of_headers", header.size_of_headers as u64);
    out.u64("entry", header.entry as u64);
    out.opt("entry_name", symbols.entry_name.as_deref());
    out.u64("image_base", header.image_base);
    out.u64("e_lfanew", header.lfanew as u64);
    out.u64("pointer_to_symbol_table", header.symoff as u64);
    out.u64("number_of_symbols", header.nsyms as u64);
    claim::write_load(
        &mut out,
        load.kind,
        load.interp.as_deref(),
        &load.needed,
        &load.runpath,
        &load.rpath,
        &load.delay,
        "pe",
        load.pie,
    );
    out.raw("symbols", &symbols_json(symbols));
    out.finish()
}

fn symbols_json(symbols: &Symbols) -> String {
    let mut out = Json::new();
    if symbols.rows {
        let rows: Vec<String> = symbols.imports.iter().map(import_json).collect();
        out.raw_array("import", &rows);
    } else {
        out.u64("import", u64::from(symbols.import_count));
    }
    out.u64("ordinal", u64::from(symbols.ordinal_count));
    out.u64("export", u64::from(symbols.export_count));
    out.u64("coff_symbols", u64::from(symbols.coff_symbols));
    out.finish_inline()
}

fn import_json(row: &Import) -> String {
    let mut one = Json::new();
    one.str("dll", &row.dll);
    if let Some(name) = &row.name {
        one.str("name", name);
    }
    if let Some(hint) = row.hint {
        one.u64("hint", u64::from(hint));
    }
    if let Some(ordinal) = row.ordinal {
        one.u64("ordinal", u64::from(ordinal));
    }
    one.finish_inline()
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
            lfanew: 0x40,
            machine: 0x8664,
            sections: 1,
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
    fn one_object_uses_pe_names() {
        let text = render("exec.exe", &exe(), &still());
        assert!(text.contains(r#""format":"pe""#), "{text}");
        assert!(text.contains(r#""magic":"MZ""#), "{text}");
        assert!(text.contains(r#""class":"PE32+""#), "{text}");
        assert!(text.contains(r#""machine_name":"x86-64""#), "{text}");
        assert!(
            text.contains(r#""load":{"kind":"static-pie","interp":null,"needed":[],"runpath":[],"rpath":[],"delay":[],"dlopen":null,"pie":true}"#),
            "{text}"
        );
        assert!(text.contains(r#""entry_name":null"#), "{text}");
        assert!(
            text.contains(r#""symbols":{"import":0,"ordinal":0,"export":0,"coff_symbols":0}"#),
            "{text}"
        );
        assert!(!text.contains(r#""os_version""#), "{text}");
        assert_eq!(text.matches('{').count(), 3, "{text}");
    }
}
