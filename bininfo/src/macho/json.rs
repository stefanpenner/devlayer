/*
---
why: This file writes one Mach-O header as one JSON object.
uses: bininfo
how: bininfo calls this file after it reads the header.
A needed object includes the import count for that library.
This file does not read Mach-O bytes.
---
*/

//! One JSON object for one Mach-O header.
//! This file does not read Mach-O bytes.

use super::header::{Fat, Image, Thin};
use super::load::{self, Commands, Slice};
use super::symbols::{self, BindImport, ChainedImport};
use super::words::{
    class_name, cpu_name, data_name, file_name, platform_name, subtype_name, universal_word,
};
use crate::claim;
use crate::style::Json;

pub fn render(file: &str, image: &Image, commands: &Commands, slices: &[Slice]) -> String {
    match image {
        Image::Thin(thin) => thin_json(file, thin, commands),
        Image::Fat(fat) => fat_json(file, fat, commands, slices),
    }
}

fn thin_json(file: &str, thin: &Thin, commands: &Commands) -> String {
    let mut out = Json::new();
    out.str("file", file);
    out.str("format", "macho");
    out.u64("magic", thin.magic as u64);
    out.str("class", class_name(thin.class));
    out.str("data", data_name(thin.data));
    out.u64("cputype", thin.cputype as u64);
    out.opt("cputype_name", cpu_name(thin.cputype));
    out.u64("cpusubtype", thin.cpusubtype as u64);
    out.opt(
        "cpusubtype_name",
        subtype_name(thin.cputype, thin.cpusubtype),
    );
    out.str("filetype", &file_name(thin.filetype));
    out.u64("flags", thin.flags as u64);
    out.u64("header_size", thin.class.header_bytes() as u64);
    out.u64("ncmds", thin.ncmds as u64);
    out.u64("sizeofcmds", thin.sizeofcmds as u64);
    out.u64("entry", commands.entry);
    out.opt("entry_name", commands.extra.entry_name.as_deref());
    out.u64("cmdoff", thin.class.header_bytes() as u64);
    out.u64("symoff", commands.symoff as u64);
    out.u64("nsyms", commands.nsyms as u64);
    out.u64("stroff", commands.stroff as u64);
    out.u64("strsize", commands.strsize as u64);
    write_load(&mut out, commands, false);
    write_records(&mut out, commands);
    out.finish()
}

fn fat_json(file: &str, fat: &Fat, commands: &Commands, slices: &[Slice]) -> String {
    let mut out = Json::new();
    out.str("file", file);
    out.str("format", "macho");
    out.u64("magic", fat.magic as u64);
    out.str("class", class_name(fat.class));
    out.opt(
        "universal",
        universal_word(slices.iter().map(|slice| slice.cputype)),
    );
    out.str("data", data_name(fat.data));
    out.u64("nfat_arch", fat.nfat_arch as u64);
    out.u64("header_size", fat.class.header_bytes() as u64);
    out.u64("arch_size", fat.class.arch_bytes() as u64);
    let objects: Vec<String> = slices.iter().map(slice_json).collect();
    out.raw_array("slices", &objects);
    if load::shared_load(slices).is_some() {
        write_load(&mut out, commands, true);
    }
    out.finish()
}

fn slice_json(slice: &Slice) -> String {
    let mut out = Json::new();
    out.u64("cputype", u64::from(slice.cputype));
    out.opt("cputype_name", cpu_name(slice.cputype));
    out.u64("cpusubtype", u64::from(slice.cpusubtype));
    out.opt(
        "cpusubtype_name",
        subtype_name(slice.cputype, slice.cpusubtype),
    );
    out.u64("offset", slice.offset);
    out.u64("size", slice.size);
    match &slice.thin {
        Some(thin) => {
            out.str("filetype", &file_name(thin.filetype));
            out.u64("flags", u64::from(thin.flags));
        }
        None => {
            out.opt("filetype", None);
            out.u64("flags", 0);
        }
    }
    out.opt("entry_name", slice.commands.extra.entry_name.as_deref());
    write_load(&mut out, &slice.commands, false);
    write_records(&mut out, &slice.commands);
    out.finish_inline()
}

fn write_records(out: &mut Json, commands: &Commands) {
    let extra = &commands.extra;
    if let Some(build) = extra.build {
        let mut obj = Json::new();
        obj.u64("platform", u64::from(build.platform));
        obj.opt("platform_name", platform_name(build.platform));
        obj.str("minos", &symbols::dotted(build.minos));
        obj.str("sdk", &symbols::dotted(build.sdk));
        obj.str("source", build.source);
        out.raw("build", &obj.finish_inline());
    }
    let mut symbols_out = Json::new();
    if let Some(count) = extra.chained {
        if extra.rows {
            let rows: Vec<String> = extra.chained_rows.iter().map(chained_json).collect();
            symbols_out.raw_array("chained_fixups", &rows);
        } else {
            symbols_out.u64("chained_fixups", u64::from(count));
        }
    }
    if let Some(count) = extra.bind {
        if extra.rows {
            let rows: Vec<String> = extra.bind_rows.iter().map(bind_json).collect();
            symbols_out.raw_array("bind", &rows);
        } else {
            symbols_out.u64("bind", u64::from(count));
        }
    }
    if extra.rows {
        symbols_out.strings("exports", &extra.export_names);
    } else {
        symbols_out.u64("exports", u64::from(extra.exports));
    }
    symbols_out.flag("symtab", extra.symtab);
    out.raw("symbols", &symbols_out.finish_inline());
}

fn chained_json(row: &ChainedImport) -> String {
    let mut one = Json::new();
    one.u64("ordinal", u64::from(row.ordinal));
    one.str("name", &row.name);
    one.flag("weak", row.weak);
    one.finish_inline()
}

fn bind_json(row: &BindImport) -> String {
    let mut one = Json::new();
    one.u64("ordinal", u64::from(row.ordinal));
    one.str("name", &row.name);
    one.finish_inline()
}

fn write_load(out: &mut Json, commands: &Commands, names_only: bool) {
    let names = commands.load.names();
    let mut one = Json::new();
    one.str("kind", load::kind_name(commands.load.kind));
    one.opt("interp", commands.load.interp.as_deref());
    if names_only {
        one.strings("needed", &names);
    } else {
        let rows: Vec<String> = commands
            .load
            .needed
            .iter()
            .enumerate()
            .map(|(index, row)| {
                let used = commands.extra.uses.get(index + 1).copied().unwrap_or(0);
                let names = if commands.extra.rows {
                    symbols::import_names(&commands.extra, (index + 1) as u32)
                        .into_iter()
                        .map(str::to_string)
                        .collect()
                } else {
                    Vec::new()
                };
                needed_json(row, used, &names)
            })
            .collect();
        one.raw_array("needed", &rows);
    }
    one.strings("runpath", &commands.load.runpath);
    one.strings("rpath", &commands.load.rpath);
    one.strings("delay", &commands.load.delay);
    match claim::dlopen_fact("macho", &names) {
        Some(value) => one.flag("dlopen", value),
        None => one.raw("dlopen", "null"),
    }
    one.flag("pie", commands.load.pie);
    out.raw("load", &one.finish_inline());
}

fn needed_json(row: &load::Dylib, used: u32, names: &[String]) -> String {
    let mut one = Json::new();
    one.str("name", &row.name);
    if let Some(word) = row.cmd.word() {
        one.str("cmd", word);
    }
    one.str("compat", &row.compat);
    one.str("current", &row.current);
    one.u64("used", u64::from(used));
    if !names.is_empty() {
        one.strings("symbols", names);
    }
    one.finish_inline()
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
            entry: 0,
            entry_is_offset: false,
            symoff: 0,
            nsyms: 0,
            stroff: 0,
            strsize: 0,
            extra: load::Extra::empty(),
        }
    }

    #[test]
    fn one_object_uses_macho_names() {
        let text = render("exec", &exec64(), &still(), &[]);
        assert!(text.contains(r#""format":"macho""#), "{text}");
        assert!(text.contains(r#""class":"MH64""#), "{text}");
        assert!(!text.contains(r#""universal""#), "{text}");
        assert!(text.contains(r#""filetype":"EXECUTE""#), "{text}");
        assert!(text.contains(r#""cputype_name":"AArch64""#), "{text}");
        assert!(text.contains(r#""cpusubtype_name":"ARM64_ALL""#), "{text}");
        assert!(
            text.contains(r#""load":{"kind":"static","interp":null,"needed":[],"runpath":[],"rpath":[],"delay":[],"dlopen":null,"pie":false}"#),
            "{text}"
        );
        assert!(text.contains(r#""entry_name":null"#), "{text}");
        assert!(
            text.contains(r#""symbols":{"exports":0,"symtab":false}"#),
            "{text}"
        );
        assert!(!text.contains(r#""build""#), "{text}");
        assert!(!text.contains(r#""dylib""#), "{text}");
        assert!(!text.contains(r#""chained_fixups""#), "{text}");
        assert!(!text.contains(r#""bind""#), "{text}");
        assert_eq!(text.matches('{').count(), 3, "{text}");
    }

    #[test]
    fn needed_rows_keep_versions_and_omit_a_load_cmd() {
        let mut commands = still();
        commands.load.needed = vec![
            load::Dylib {
                cmd: load::DylibCmd::Load,
                name: "liba.dylib".into(),
                compat: "1.0.0".into(),
                current: "2.0.0".into(),
            },
            load::Dylib {
                cmd: load::DylibCmd::Reexport,
                name: "libb.dylib".into(),
                compat: "3.1.0".into(),
                current: "4.2.1".into(),
            },
        ];
        let text = render("exec", &exec64(), &commands, &[]);
        assert!(
            text.contains(
                r#""needed":[{"name":"liba.dylib","compat":"1.0.0","current":"2.0.0","used":0},{"name":"libb.dylib","cmd":"reexport","compat":"3.1.0","current":"4.2.1","used":0}]"#
            ),
            "{text}"
        );
        assert!(!text.contains(r#""dylib""#), "{text}");
        assert!(!text.contains(r#""cmd":"load""#), "{text}");
    }

    #[test]
    fn a_needed_object_names_the_symbols_that_library_supplies() {
        let mut commands = still();
        commands.load.needed = vec![load::link("liba.dylib")];
        commands.extra.rows = true;
        commands.extra.chained = Some(1);
        commands.extra.uses = vec![0, 1];
        commands.extra.chained_rows = vec![ChainedImport {
            ordinal: 1,
            name: "_puts".into(),
            weak: true,
        }];
        let text = render("exec", &exec64(), &commands, &[]);
        assert!(
            text.contains(
                r#""name":"liba.dylib","compat":"0.0.0","current":"0.0.0","used":1,"symbols":["_puts"]"#
            ),
            "{text}"
        );
    }

    fn pie_slice(cpu: u32, sub: u32, needed: &str) -> Slice {
        use super::super::header::{Class, Data, FileType, Thin};
        Slice {
            cputype: cpu,
            cpusubtype: sub,
            offset: 0x4000,
            size: 64,
            thin: Some(Thin {
                class: Class::Mach64,
                data: Data::Little,
                magic: 0xfeed_facf,
                cputype: cpu,
                cpusubtype: sub,
                filetype: FileType::Execute,
                ncmds: 1,
                sizeofcmds: 0,
                flags: 0x200000,
            }),
            commands: Commands {
                load: Load {
                    kind: Kind::DynamicPie,
                    interp: Some("/usr/lib/dyld".into()),
                    needed: vec![load::link(needed)],
                    runpath: Vec::new(),
                    rpath: vec!["@executable_path/../Frameworks".into()],
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
                    cpusubtype: 2,
                    offset: 0x5000,
                    size: 64,
                },
            ],
        })
    }

    #[test]
    fn fat_json_slices_are_objects_in_file_order() {
        let slices = vec![
            pie_slice(0x0100_0007, 3, "libx.dylib"),
            pie_slice(0x0100_000c, 2, "liba.dylib"),
        ];
        let commands = load::commands_for(&slices);
        let text = render("Photo", &fat_image(), &commands, &slices);
        assert!(text.contains(r#""class":"FAT""#), "{text}");
        assert!(text.contains(r#""universal":"universal2""#), "{text}");
        assert!(text.contains(r#""cputype_name":"x86-64""#), "{text}");
        assert!(text.contains(r#""cpusubtype_name":"X86_64_ALL""#), "{text}");
        assert!(text.contains(r#""cputype_name":"AArch64""#), "{text}");
        assert!(text.contains(r#""cpusubtype_name":"ARM64E""#), "{text}");
        assert!(
            text.contains(
                r#""needed":[{"name":"libx.dylib","compat":"0.0.0","current":"0.0.0","used":0}]"#
            ),
            "{text}"
        );
        assert!(
            text.contains(
                r#""needed":[{"name":"liba.dylib","compat":"0.0.0","current":"0.0.0","used":0}]"#
            ),
            "{text}"
        );
        let x = text.find("libx.dylib").expect("libx");
        let a = text.find("liba.dylib").expect("liba");
        assert!(x < a, "{text}");
        assert!(
            text.contains(r#""rpath":["@executable_path/../Frameworks"]"#),
            "{text}"
        );
        assert!(!text.contains(r#""load":{"kind":"other""#), "{text}");
        assert!(!text.contains("This page does not open a slice"), "{text}");
    }

    #[test]
    fn matching_slices_keep_one_top_level_load() {
        let slices = vec![
            pie_slice(0x0100_0007, 3, "liba.dylib"),
            pie_slice(0x0100_000c, 0, "liba.dylib"),
        ];
        let commands = load::commands_for(&slices);
        let text = render("Photo", &fat_image(), &commands, &slices);
        assert_eq!(
            text.matches(r#""needed":["liba.dylib"]"#).count(),
            1,
            "{text}"
        );
        assert_eq!(
            text.matches(r#""name":"liba.dylib","compat":"0.0.0","current":"0.0.0","used":0"#)
                .count(),
            2,
            "{text}"
        );
        assert!(text.contains(r#""kind":"dynamic-pie""#), "{text}");
    }

    #[test]
    fn fat_json_names_universal_when_the_pair_is_absent() {
        let slices = vec![
            pie_slice(0x0100_000c, 2, "liba.dylib"),
            pie_slice(0x0100_000c, 12, "liba.dylib"),
        ];
        let text = render("Siri", &fat_image(), &still(), &slices);
        assert!(text.contains(r#""universal":"universal""#), "{text}");
        assert!(!text.contains("universal2"), "{text}");

        let empty = render("empty", &fat_image(), &still(), &[]);
        assert!(empty.contains(r#""universal":null"#), "{empty}");
    }

    #[test]
    fn a_closed_slice_says_it_does_not_open() {
        let mut slice = pie_slice(0x0100_0007, 3, "liba.dylib");
        slice.thin = None;
        slice.commands.load.kind = super::super::load::Kind::Other;
        slice.commands.load.interp = None;
        slice.commands.load.needed.clear();
        slice.commands.load.rpath.clear();
        slice.commands.load.pie = false;
        let commands = load::commands_for(std::slice::from_ref(&slice));
        let text = render("Photo", &fat_image(), &commands, &[slice]);
        assert!(text.contains(r#""filetype":null"#), "{text}");
        assert!(!text.contains("says"), "{text}");
    }
}
