/*
---
why: This file reads how a Mach-O file starts.
uses: bininfo
how: bininfo calls this file to read load commands.
This file does not print the page.
---
*/

//! How a Mach-O file starts, and which files it must open.
//! This module reads load commands. It does not print them.

use super::header::{self, Arch, Class, Data, Fat, FileType, Image, Thin};
use super::symbols;
pub use super::symbols::Extra;
pub(crate) use crate::kind::name as kind_name;
pub use crate::kind::Kind;

/// The load command for one linked library.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum DylibCmd {
    /// LC_LOAD_DYLIB.
    Load,
    /// LC_LOAD_WEAK_DYLIB.
    Weak,
    /// LC_REEXPORT_DYLIB.
    Reexport,
    /// LC_LOAD_UPWARD_DYLIB.
    Upward,
}

impl DylibCmd {
    /// Page word. `Load` has no word.
    pub(crate) fn word(self) -> Option<&'static str> {
        match self {
            Self::Load => None,
            Self::Weak => Some("weak"),
            Self::Reexport => Some("reexport"),
            Self::Upward => Some("upward"),
        }
    }
}

/// One linked library.
/// `compat` is compatibility_version. `current` is current_version.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Dylib {
    pub cmd: DylibCmd,
    pub name: String,
    pub compat: String,
    pub current: String,
}

const LC_SYMTAB: u32 = 0x2;
const LC_DYSYMTAB: u32 = 0xb;
const LC_UNIXTHREAD: u32 = 0x5;
const LC_LOAD_DYLIB: u32 = 0xc;
const LC_LOAD_DYLINKER: u32 = 0xe;
const LC_RPATH: u32 = 0x1c;
const LC_LOAD_WEAK_DYLIB: u32 = 0x18;
const LC_REEXPORT_DYLIB: u32 = 0x1f;
const LC_LOAD_UPWARD_DYLIB: u32 = 0x23;
const LC_DYLD_INFO: u32 = 0x22;
const LC_VERSION_MIN_MACOSX: u32 = 0x24;
const LC_MAIN: u32 = 0x28;
const LC_BUILD_VERSION: u32 = 0x32;
const LC_DYLD_EXPORTS_TRIE: u32 = 0x33;
const LC_DYLD_CHAINED_FIXUPS: u32 = 0x34;
const LC_REQ: u32 = 0x8000_0000;
const MH_PIE: u32 = 0x0020_0000;
const NAME_CAP: usize = 4096;

/// Facts that match the ELF load map.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Load {
    pub kind: Kind,
    pub interp: Option<String>, // LC_LOAD_DYLINKER path. None means this file starts itself.
    pub needed: Vec<Dylib>,     // One row is one load command.
    pub runpath: Vec<String>,   // Always empty. Mach-O has no RUNPATH record.
    pub rpath: Vec<String>,     // LC_RPATH directories, in file order.
    pub delay: Vec<String>,     // Always empty. Mach-O has no delay-import record.
    pub pie: bool,              // MH_PIE bit in the header flags.
}

impl Load {
    /// Library names in file order. Claims read this list.
    pub fn names(&self) -> Vec<String> {
        self.needed.iter().map(|row| row.name.clone()).collect()
    }
}

#[cfg(test)]
pub(crate) fn link(name: &str) -> Dylib {
    Dylib {
        cmd: DylibCmd::Load,
        name: name.to_string(),
        compat: "0.0.0".into(),
        current: "0.0.0".into(),
    }
}

/// Load-command facts. `entry` is not in the Mach-O header.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Commands {
    pub load: Load,
    pub entry: u64,            // LC_MAIN file offset, or a thread-state address.
    pub entry_is_offset: bool, // True when entry comes from LC_MAIN.
    pub symoff: u32,           // LC_SYMTAB. The loader does not need this.
    pub nsyms: u32,
    pub stroff: u32,
    pub strsize: u32,
    pub extra: Extra, // Build version and loader symbols.
}

/// One fat slice. `thin` is absent when those bytes are not a thin Mach-O.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Slice {
    pub cputype: u32,
    pub cpusubtype: u32,
    pub offset: u64,
    pub size: u64,
    pub thin: Option<Thin>,
    pub commands: Commands,
}

#[cfg(test)]
pub fn read(bytes: &[u8], image: &Image) -> Commands {
    read_at(bytes, image, false)
}

pub fn read_at(bytes: &[u8], image: &Image, rows: bool) -> Commands {
    match image {
        Image::Thin(thin) => read_thin(bytes, thin, rows),
        Image::Fat(fat) => commands_for(&open_at(bytes, fat, rows)),
    }
}

pub fn slices_rows(bytes: &[u8], image: &Image, rows: bool) -> Vec<Slice> {
    match image {
        Image::Thin(_) => Vec::new(),
        Image::Fat(fat) => open_at(bytes, fat, rows),
    }
}

/// Open each slice. The window is a borrow. A large slice is not copied.
#[cfg(test)]
pub fn open_slices(bytes: &[u8], fat: &Fat) -> Vec<Slice> {
    open_at(bytes, fat, false)
}

fn open_at(bytes: &[u8], fat: &Fat, rows: bool) -> Vec<Slice> {
    fat.arches
        .iter()
        .map(|arch| open_one(bytes, arch, rows))
        .collect()
}

fn open_one(bytes: &[u8], arch: &Arch, rows: bool) -> Slice {
    let (thin, commands) = match slice_window(bytes, arch.offset, arch.size) {
        Some(window) => match header::parse(window) {
            Ok(Image::Thin(thin)) => {
                let commands = read_thin(window, &thin, rows);
                (Some(thin), commands)
            }
            _ => (None, blank(Kind::Other)),
        },
        None => (None, blank(Kind::Other)),
    };
    Slice {
        cputype: arch.cputype,
        cpusubtype: arch.cpusubtype,
        offset: arch.offset,
        size: arch.size,
        thin,
        commands,
    }
}

fn slice_window(bytes: &[u8], offset: u64, size: u64) -> Option<&[u8]> {
    if size == 0 {
        return None;
    }
    let start = usize::try_from(offset).ok()?;
    if start >= bytes.len() {
        return None;
    }
    let want = usize::try_from(size).unwrap_or(usize::MAX);
    let end = start.saturating_add(want).min(bytes.len());
    if end <= start {
        return None;
    }
    Some(&bytes[start..end])
}

/// The load when every slice has the same load facts.
pub fn shared_load(slices: &[Slice]) -> Option<Load> {
    let first = slices.first()?;
    if slices
        .iter()
        .skip(1)
        .all(|slice| same_load(&slice.commands.load, &first.commands.load))
    {
        Some(first.commands.load.clone())
    } else {
        None
    }
}

/// None when the slices share one load.
/// Some is the hermetic claim when they do not.
pub fn fat_hermetic(slices: &[Slice]) -> Option<bool> {
    if slices.is_empty() || shared_load(slices).is_some() {
        return None;
    }
    Some(slices.iter().all(|slice| {
        slice.thin.is_some()
            && crate::claim::is_hermetic(
                slice.commands.load.kind,
                slice.commands.load.interp.as_deref(),
                &slice.commands.load.names(),
            )
    }))
}

pub fn commands_for(slices: &[Slice]) -> Commands {
    if let Some(first) = slices.first() {
        if shared_load(slices).is_some() {
            return first.commands.clone();
        }
    }
    let mut commands = blank(Kind::Other);
    commands.load.pie = !slices.is_empty()
        && slices
            .iter()
            .all(|slice| slice.thin.is_some() && slice.commands.load.pie);
    commands
}

fn same_names(left: &[Dylib], right: &[Dylib]) -> bool {
    left.len() == right.len() && left.iter().zip(right).all(|(a, b)| a.name == b.name)
}

fn same_load(left: &Load, right: &Load) -> bool {
    left.kind == right.kind
        && left.interp == right.interp
        && same_names(&left.needed, &right.needed)
        && left.runpath == right.runpath
        && left.rpath == right.rpath
        && left.delay == right.delay
        && left.pie == right.pie
}

fn read_thin(bytes: &[u8], thin: &Thin, rows: bool) -> Commands {
    let mut commands = blank(Kind::Other);
    commands.extra.rows = rows;
    let mut saw_main = false;
    let mut version_min = None;
    let mut info_bind = None;
    let mut info_exports = None;
    let mut trie_exports = None;
    let mut undef_range = None;
    let start = thin.class.header_bytes() as usize;
    let end = start
        .saturating_add(thin.sizeofcmds as usize)
        .min(bytes.len());
    let mut at = start;
    let mut seen = 0u32;
    while seen < thin.ncmds {
        let Some(cmd) = header::u32_at(bytes, at, thin.data) else {
            break;
        };
        let Some(size) = header::u32_at(bytes, at + 4, thin.data) else {
            break;
        };
        if size < 8 {
            break;
        }
        let Some(next) = at.checked_add(size as usize) else {
            break;
        };
        if next > end || next > bytes.len() {
            break;
        }
        match cmd & !LC_REQ {
            LC_LOAD_DYLINKER => {
                if let Some(path) = cmd_name(bytes, at, size, thin.data) {
                    commands.load.interp = Some(path);
                }
            }
            LC_LOAD_DYLIB | LC_LOAD_WEAK_DYLIB | LC_REEXPORT_DYLIB | LC_LOAD_UPWARD_DYLIB => {
                if let Some(name) = cmd_name(bytes, at, size, thin.data) {
                    let (compat, current) = symbols::dylib_versions(bytes, at, size, thin.data);
                    let cmd = match cmd & !LC_REQ {
                        LC_LOAD_WEAK_DYLIB => DylibCmd::Weak,
                        LC_REEXPORT_DYLIB => DylibCmd::Reexport,
                        LC_LOAD_UPWARD_DYLIB => DylibCmd::Upward,
                        _ => DylibCmd::Load,
                    };
                    commands.load.needed.push(Dylib {
                        cmd,
                        name,
                        compat,
                        current,
                    });
                }
            }
            LC_RPATH => {
                if let Some(path) = cmd_name(bytes, at, size, thin.data) {
                    commands.load.rpath.push(path);
                }
            }
            LC_MAIN => {
                if let Some(entry) = header::u64_at(bytes, at + 8, thin.data) {
                    commands.entry = entry;
                    commands.entry_is_offset = true;
                    saw_main = true;
                }
            }
            LC_UNIXTHREAD if !saw_main => {
                if let Some(pc) = thread_pc(bytes, at, size, thin) {
                    commands.entry = pc;
                    commands.entry_is_offset = false;
                }
            }
            LC_SYMTAB if size >= 24 => {
                commands.symoff = header::u32_at(bytes, at + 8, thin.data).unwrap_or(0);
                commands.nsyms = header::u32_at(bytes, at + 12, thin.data).unwrap_or(0);
                commands.stroff = header::u32_at(bytes, at + 16, thin.data).unwrap_or(0);
                commands.strsize = header::u32_at(bytes, at + 20, thin.data).unwrap_or(0);
            }
            LC_DYSYMTAB if size >= 32 => {
                let start = header::u32_at(bytes, at + 24, thin.data).unwrap_or(0);
                let count = header::u32_at(bytes, at + 28, thin.data).unwrap_or(0);
                undef_range = Some((start, count));
            }
            LC_BUILD_VERSION => {
                if commands.extra.build.is_none() {
                    commands.extra.build = symbols::build_version(bytes, at, size, thin.data);
                }
            }
            LC_VERSION_MIN_MACOSX => {
                if version_min.is_none() {
                    version_min = symbols::version_min(bytes, at, size, thin.data);
                }
            }
            LC_DYLD_CHAINED_FIXUPS if size >= 16 => {
                let dataoff = header::u32_at(bytes, at + 8, thin.data).unwrap_or(0);
                let datasize = header::u32_at(bytes, at + 12, thin.data).unwrap_or(0);
                let (count, found, uses) =
                    symbols::chained(bytes, dataoff, datasize, thin.data, rows);
                commands.extra.chained = Some(count);
                commands.extra.chained_rows = found;
                commands.extra.uses = uses;
            }
            LC_DYLD_INFO if size >= 48 => {
                let bind_off = header::u32_at(bytes, at + 16, thin.data).unwrap_or(0);
                let bind_size = header::u32_at(bytes, at + 20, thin.data).unwrap_or(0);
                let export_off = header::u32_at(bytes, at + 40, thin.data).unwrap_or(0);
                let export_size = header::u32_at(bytes, at + 44, thin.data).unwrap_or(0);
                info_bind = Some(symbols::bind(bytes, bind_off, bind_size, thin.data, rows));
                info_exports = Some(symbols::exports(bytes, export_off, export_size, rows));
            }
            LC_DYLD_EXPORTS_TRIE if size >= 16 => {
                let dataoff = header::u32_at(bytes, at + 8, thin.data).unwrap_or(0);
                let datasize = header::u32_at(bytes, at + 12, thin.data).unwrap_or(0);
                trie_exports = Some(symbols::exports(bytes, dataoff, datasize, rows));
            }
            _ => {}
        }
        at = next;
        seen += 1;
    }
    let pie = thin.flags & MH_PIE != 0;
    commands.load.pie = pie;
    commands.load.kind = classify(thin.filetype, commands.load.interp.is_some(), pie);
    if commands.extra.build.is_none() {
        commands.extra.build = version_min;
    }
    if commands.extra.chained.is_none() {
        if let Some((count, found, uses)) = info_bind {
            commands.extra.bind = Some(count);
            commands.extra.bind_rows = found;
            commands.extra.uses = uses;
        }
    }
    let from_trie = trie_exports.is_some();
    if let Some((count, names)) = trie_exports.or(info_exports) {
        commands.extra.exports = count;
        commands.extra.export_names = names;
        commands.extra.export_cmd = if from_trie {
            "LC_DYLD_EXPORTS_TRIE"
        } else {
            "LC_DYLD_INFO"
        };
    }
    commands.extra.symtab = commands.nsyms > 0 && commands.symoff > 0;
    if let Some(name) = symbols::nlist_entry(
        bytes,
        thin,
        commands.symoff,
        commands.nsyms,
        commands.stroff,
        commands.strsize,
        commands.entry,
    ) {
        commands.extra.entry_name = Some(name);
        commands.extra.entry_note = "[LC_SYMTAB] Symbol whose value equals the entry.";
    }
    let (count, found, uses) = symbols::undefined(
        bytes,
        thin,
        commands.symoff,
        commands.nsyms,
        commands.stroff,
        commands.strsize,
        undef_range,
        rows,
    );
    if count > 0 {
        commands.extra.undef = Some(count);
        commands.extra.undef_rows = found;
        commands.extra.uses = uses;
    }
    commands
}

pub(crate) fn kind_note(kind: Kind) -> &'static str {
    match kind {
        Kind::Static => "MH_EXECUTE and no loader.",
        Kind::Dynamic => "A loader is present and the PIE bit is clear.",
        Kind::DynamicPie => "A loader is present and the PIE bit is set.",
        Kind::StaticPie => "MH_EXECUTE, no loader, and the PIE bit is set.",
        Kind::Shared => "A shared library or a bundle.",
        Kind::Rel => "MH_OBJECT. The kernel does not run this file.",
        Kind::Other => "This file type is outside the four start kinds.",
    }
}

fn classify(ty: FileType, interp: bool, pie: bool) -> Kind {
    match ty {
        FileType::Object => Kind::Rel,
        FileType::Execute if interp && pie => Kind::DynamicPie,
        FileType::Execute if interp => Kind::Dynamic,
        FileType::Execute if pie => Kind::StaticPie,
        FileType::Execute => Kind::Static,
        FileType::Dylib | FileType::Bundle => Kind::Shared,
        _ => Kind::Other,
    }
}

fn blank(kind: Kind) -> Commands {
    Commands {
        load: Load {
            kind,
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
        extra: Extra::empty(),
    }
}

fn cmd_name(bytes: &[u8], at: usize, size: u32, data: Data) -> Option<String> {
    let off = header::u32_at(bytes, at + 8, data)?;
    let start = at.checked_add(off as usize)?;
    let end = at.checked_add(size as usize)?.min(bytes.len());
    if start >= end {
        return None;
    }
    let take = (end - start).min(NAME_CAP);
    let slice = &bytes[start..start + take];
    let n = slice.iter().position(|b| *b == 0).unwrap_or(slice.len());
    let text = String::from_utf8_lossy(&slice[..n]).into_owned();
    if text.is_empty() {
        None
    } else {
        Some(text)
    }
}

fn thread_pc(bytes: &[u8], at: usize, size: u32, thin: &Thin) -> Option<u64> {
    let flavor = header::u32_at(bytes, at + 8, thin.data)?;
    let state = at + 16;
    let end = at + size as usize;
    let read_u64 = |pos: usize| {
        if pos + 8 > end {
            None
        } else {
            header::u64_at(bytes, pos, thin.data)
        }
    };
    match flavor {
        4 => read_u64(state + 128),
        6 => read_u64(state + 256),
        1 if thin.class == Class::Mach32 => {
            if state + 44 > end {
                None
            } else {
                header::u32_at(bytes, state + 40, thin.data).map(|v| v as u64)
            }
        }
        _ => None,
    }
}

#[cfg(test)]
mod tests {
    use super::super::header::{Data, FileType};
    use super::*;

    fn put_u32(buf: &mut [u8], at: usize, v: u32, data: Data) {
        let b = match data {
            Data::Little => v.to_le_bytes(),
            Data::Big => v.to_be_bytes(),
        };
        buf[at..at + 4].copy_from_slice(&b);
    }

    fn put_u64(buf: &mut [u8], at: usize, v: u64, data: Data) {
        let b = match data {
            Data::Little => v.to_le_bytes(),
            Data::Big => v.to_be_bytes(),
        };
        buf[at..at + 8].copy_from_slice(&b);
    }

    fn header(filetype: u32, flags: u32, data: Data) -> Vec<u8> {
        let mut b = vec![0u8; 32];
        let magic: [u8; 4] = match data {
            Data::Little => [0xcf, 0xfa, 0xed, 0xfe],
            Data::Big => [0xfe, 0xed, 0xfa, 0xcf],
        };
        b[..4].copy_from_slice(&magic);
        put_u32(&mut b, 4, 0x0100_0007, data);
        put_u32(&mut b, 12, filetype, data);
        put_u32(&mut b, 24, flags, data);
        b
    }

    fn finish(buf: &mut Vec<u8>, data: Data, ncmds: u32) {
        let size = (buf.len() - 32) as u32;
        put_u32(buf, 16, ncmds, data);
        put_u32(buf, 20, size, data);
    }

    fn push_text(buf: &mut Vec<u8>, cmd: u32, name_off: u32, text: &str, data: Data) {
        let raw = name_off + text.len() as u32 + 1;
        let size = (raw + 7) & !7;
        let at = buf.len();
        buf.resize(at + size as usize, 0);
        put_u32(buf, at, cmd, data);
        put_u32(buf, at + 4, size, data);
        put_u32(buf, at + 8, name_off, data);
        let start = at + name_off as usize;
        buf[start..start + text.len()].copy_from_slice(text.as_bytes());
    }

    fn push_main(buf: &mut Vec<u8>, entry: u64, data: Data) {
        let at = buf.len();
        buf.resize(at + 24, 0);
        put_u32(buf, at, LC_MAIN | LC_REQ, data);
        put_u32(buf, at + 4, 24, data);
        put_u64(buf, at + 8, entry, data);
    }

    #[test]
    fn static_execute_names_no_loader() {
        let bytes = header(2, 0, Data::Little);
        let image = header::parse(&bytes).unwrap();
        let commands = read(&bytes, &image);
        assert_eq!(commands.load.kind, Kind::Static);
        assert!(commands.load.interp.is_none());
        assert!(commands.load.needed.is_empty());
        assert!(!commands.load.pie);
    }

    #[test]
    fn static_pie_sets_the_flag_and_names_no_loader() {
        let bytes = header(2, MH_PIE, Data::Little);
        let image = header::parse(&bytes).unwrap();
        let commands = read(&bytes, &image);
        assert_eq!(commands.load.kind, Kind::StaticPie);
        assert!(commands.load.pie);
    }

    #[test]
    fn dynamic_pie_names_the_loader_and_two_libraries() {
        let mut bytes = header(2, MH_PIE, Data::Little);
        push_text(
            &mut bytes,
            LC_LOAD_DYLINKER,
            12,
            "/usr/lib/dyld",
            Data::Little,
        );
        push_text(&mut bytes, LC_LOAD_DYLIB, 24, "liba.dylib", Data::Little);
        push_text(
            &mut bytes,
            LC_LOAD_DYLIB | LC_REQ,
            24,
            "libb.dylib",
            Data::Little,
        );
        push_main(&mut bytes, 0x1000, Data::Little);
        finish(&mut bytes, Data::Little, 4);
        let image = header::parse(&bytes).unwrap();
        let commands = read(&bytes, &image);
        assert_eq!(commands.load.kind, Kind::DynamicPie);
        assert_eq!(commands.load.interp.as_deref(), Some("/usr/lib/dyld"));
        assert_eq!(commands.load.names(), ["liba.dylib", "libb.dylib"]);
        assert!(commands
            .load
            .needed
            .iter()
            .all(|row| row.cmd == DylibCmd::Load));
        assert!(commands.load.pie);
        assert!(commands.entry_is_offset);
        assert_eq!(commands.entry, 0x1000);
    }

    #[test]
    fn os_load_allows_libsystem_and_rejects_iconv() {
        let mut bytes = header(2, MH_PIE, Data::Little);
        push_text(
            &mut bytes,
            LC_LOAD_DYLINKER,
            12,
            "/usr/lib/dyld",
            Data::Little,
        );
        push_text(
            &mut bytes,
            LC_LOAD_DYLIB,
            24,
            "/usr/lib/libSystem.B.dylib",
            Data::Little,
        );
        finish(&mut bytes, Data::Little, 2);
        let file = crate::open(&bytes, false).unwrap();
        let set = crate::policy::parse_set(&[(
            "bin/x",
            "format = \"macho\"\ncputype = [\"CPU_TYPE_X86_64\"]\nfiletype = \"MH_EXECUTE\"\nMH_PIE = true\nLC_LOAD_DYLINKER = \"/usr/lib/dyld\"\nLC_LOAD_DYLIB = [{ name = \"/usr/lib/libSystem.B.dylib\", why = \"macOS has no static libc.\" }]\n",
        )])
        .unwrap();
        assert!(crate::osload::judge("bin/x", &file, &set).is_ok());

        push_text(
            &mut bytes,
            LC_LOAD_DYLIB,
            24,
            "/usr/lib/libiconv.2.dylib",
            Data::Little,
        );
        finish(&mut bytes, Data::Little, 3);
        let file = crate::open(&bytes, false).unwrap();
        let err = crate::osload::judge("bin/x", &file, &set).unwrap_err();
        assert!(err.contains("/usr/lib/libiconv.2.dylib"), "{err}");
    }

    #[test]
    fn dynamic_without_the_pie_bit() {
        let mut bytes = header(2, 0, Data::Little);
        push_text(
            &mut bytes,
            LC_LOAD_DYLINKER,
            12,
            "/usr/lib/dyld",
            Data::Little,
        );
        finish(&mut bytes, Data::Little, 1);
        let image = header::parse(&bytes).unwrap();
        let commands = read(&bytes, &image);
        assert_eq!(commands.load.kind, Kind::Dynamic);
        assert!(!commands.load.pie);
    }

    #[test]
    fn shared_library_keeps_its_needed_list() {
        let mut bytes = header(6, 0, Data::Little);
        push_text(
            &mut bytes,
            LC_LOAD_DYLIB,
            24,
            "/usr/lib/libSystem.B.dylib",
            Data::Little,
        );
        finish(&mut bytes, Data::Little, 1);
        let image = header::parse(&bytes).unwrap();
        let commands = read(&bytes, &image);
        assert_eq!(commands.load.kind, Kind::Shared);
        assert!(commands.load.interp.is_none());
        assert_eq!(commands.load.names(), ["/usr/lib/libSystem.B.dylib"]);
    }

    #[test]
    fn rpath_commands_keep_file_order() {
        let mut bytes = header(2, MH_PIE, Data::Little);
        push_text(
            &mut bytes,
            LC_RPATH | LC_REQ,
            12,
            "@executable_path/../lib",
            Data::Little,
        );
        push_text(&mut bytes, LC_RPATH | LC_REQ, 12, "/opt/lib", Data::Little);
        finish(&mut bytes, Data::Little, 2);
        let image = header::parse(&bytes).unwrap();
        let commands = read(&bytes, &image);
        assert!(commands.load.runpath.is_empty());
        assert_eq!(commands.load.rpath, ["@executable_path/../lib", "/opt/lib"]);
    }

    #[test]
    fn object_file_is_rel_and_thread_state_is_an_address() {
        let mut bytes = header(1, 0, Data::Little);
        let at = bytes.len();
        bytes.resize(at + 16 + 136, 0);
        put_u32(&mut bytes, at, LC_UNIXTHREAD, Data::Little);
        put_u32(&mut bytes, at + 4, (16 + 136) as u32, Data::Little);
        put_u32(&mut bytes, at + 8, 4, Data::Little);
        put_u32(&mut bytes, at + 12, 42, Data::Little);
        put_u64(
            &mut bytes,
            at + 16 + 128,
            0x1122_3344_5566_7788,
            Data::Little,
        );
        finish(&mut bytes, Data::Little, 1);
        let image = header::parse(&bytes).unwrap();
        let commands = read(&bytes, &image);
        assert_eq!(commands.load.kind, Kind::Rel);
        assert!(!commands.entry_is_offset);
        assert_eq!(commands.entry, 0x1122_3344_5566_7788);
    }

    #[test]
    fn unixthread_flavor_is_the_word_at_offset_8() {
        let mut bytes = header(1, 0, Data::Little);
        let at = bytes.len();
        bytes.resize(at + 16 + 136, 0);
        put_u32(&mut bytes, at, LC_UNIXTHREAD, Data::Little);
        put_u32(&mut bytes, at + 4, (16 + 136) as u32, Data::Little);
        put_u32(&mut bytes, at + 8, 4, Data::Little);
        put_u32(&mut bytes, at + 12, 42, Data::Little);
        put_u64(
            &mut bytes,
            at + 16 + 128,
            0x1122_3344_5566_7788,
            Data::Little,
        );
        finish(&mut bytes, Data::Little, 1);
        let image = header::parse(&bytes).unwrap();
        let commands = read(&bytes, &image);
        assert_eq!(commands.entry, 0x1122_3344_5566_7788);
    }

    #[test]
    fn nlist_name_stops_at_strsize() {
        let mut bytes = header(2, 0, Data::Little);
        push_main(&mut bytes, 0x1000, Data::Little);
        let cmd = bytes.len();
        bytes.resize(cmd + 24, 0);
        put_u32(&mut bytes, cmd, LC_SYMTAB, Data::Little);
        put_u32(&mut bytes, cmd + 4, 24, Data::Little);
        let symoff = (cmd + 24) as u32;
        put_u32(&mut bytes, cmd + 8, symoff, Data::Little);
        put_u32(&mut bytes, cmd + 12, 1, Data::Little);
        put_u32(&mut bytes, cmd + 16, symoff + 16, Data::Little);
        put_u32(&mut bytes, cmd + 20, 2, Data::Little);
        finish(&mut bytes, Data::Little, 2);
        let sym = bytes.len();
        bytes.resize(sym + 16, 0);
        put_u32(&mut bytes, sym, 0, Data::Little);
        put_u64(&mut bytes, sym + 8, 0x1000, Data::Little);
        bytes.extend_from_slice(b"hiXX\0");
        let image = header::parse(&bytes).unwrap();
        let commands = read(&bytes, &image);
        assert_eq!(commands.extra.entry_name.as_deref(), Some("hi"));
    }

    #[test]
    fn big_endian_dynamic_keeps_the_pie_bit() {
        let mut bytes = header(2, MH_PIE, Data::Big);
        push_text(&mut bytes, LC_LOAD_DYLINKER, 12, "/usr/lib/dyld", Data::Big);
        push_main(&mut bytes, 0x20, Data::Big);
        finish(&mut bytes, Data::Big, 2);
        let image = header::parse(&bytes).unwrap();
        let Image::Thin(thin) = &image else {
            panic!("fat")
        };
        assert_eq!(thin.data, Data::Big);
        assert_eq!(thin.filetype, FileType::Execute);
        let commands = read(&bytes, &image);
        assert_eq!(commands.load.kind, Kind::DynamicPie);
        assert_eq!(commands.entry, 0x20);
    }

    #[test]
    fn hostile_load_commands_do_not_panic() {
        let mut bytes = header(2, 0, Data::Little);
        put_u32(&mut bytes, 16, u32::MAX, Data::Little);
        put_u32(&mut bytes, 20, u32::MAX, Data::Little);
        let image = header::parse(&bytes).unwrap();
        assert!(std::panic::catch_unwind(|| read(&bytes, &image)).is_ok());
        let mut zero = header(2, 0, Data::Little);
        zero.extend_from_slice(&[0u8; 8]);
        put_u32(&mut zero, 16, 4, Data::Little);
        put_u32(&mut zero, 20, 8, Data::Little);
        let image = header::parse(&zero).unwrap();
        assert!(std::panic::catch_unwind(|| read(&zero, &image)).is_ok());
    }

    fn dyn_image(cpu: u32, lib: &str, rpath: &str) -> Vec<u8> {
        let mut bytes = header(2, MH_PIE, Data::Little);
        put_u32(&mut bytes, 4, cpu, Data::Little);
        push_text(
            &mut bytes,
            LC_LOAD_DYLINKER,
            12,
            "/usr/lib/dyld",
            Data::Little,
        );
        push_text(&mut bytes, LC_LOAD_WEAK_DYLIB, 24, lib, Data::Little);
        push_text(&mut bytes, LC_RPATH | LC_REQ, 12, rpath, Data::Little);
        finish(&mut bytes, Data::Little, 3);
        bytes
    }

    fn fat_of(parts: &[(u32, u32, Vec<u8>)]) -> Vec<u8> {
        let mut bytes = vec![0u8; 8 + 20 * parts.len()];
        bytes[..4].copy_from_slice(&[0xca, 0xfe, 0xba, 0xbe]);
        bytes[4..8].copy_from_slice(&(parts.len() as u32).to_be_bytes());
        for (index, (cpu, sub, image)) in parts.iter().enumerate() {
            let at = 8 + 20 * index;
            let cursor = bytes.len() as u32;
            bytes[at..at + 4].copy_from_slice(&cpu.to_be_bytes());
            bytes[at + 4..at + 8].copy_from_slice(&sub.to_be_bytes());
            bytes[at + 8..at + 12].copy_from_slice(&cursor.to_be_bytes());
            bytes[at + 12..at + 16].copy_from_slice(&(image.len() as u32).to_be_bytes());
            bytes.extend_from_slice(image);
        }
        bytes
    }

    #[test]
    fn fat_opens_each_slice_in_file_order() {
        let x86 = dyn_image(0x0100_0007, "libx.dylib", "@executable_path/x:y");
        let arm = dyn_image(0x0100_000c, "liba.dylib", "@executable_path/a");
        let bytes = fat_of(&[(0x0100_0007, 3, x86), (0x0100_000c, 2, arm)]);
        let image = header::parse(&bytes).unwrap();
        let Image::Fat(fat) = &image else {
            panic!("thin")
        };
        let slices = open_slices(&bytes, fat);
        assert_eq!(slices.len(), 2);
        assert_eq!(slices[0].cputype, 0x0100_0007);
        assert_eq!(slices[0].cpusubtype, 3);
        assert_eq!(slices[0].commands.load.kind, Kind::DynamicPie);
        assert_eq!(
            slices[0].commands.load.interp.as_deref(),
            Some("/usr/lib/dyld")
        );
        assert_eq!(slices[0].commands.load.names(), ["libx.dylib"]);
        assert_eq!(slices[0].commands.load.rpath, ["@executable_path/x:y"]);
        assert!(slices[0].commands.load.runpath.is_empty());
        assert!(slices[0].commands.load.pie);
        assert!(slices[0].thin.is_some());
        assert_eq!(slices[1].cputype, 0x0100_000c);
        assert_eq!(slices[1].cpusubtype, 2);
        assert_eq!(slices[1].commands.load.names(), ["liba.dylib"]);
        assert_eq!(slices[1].commands.load.rpath, ["@executable_path/a"]);
        assert!(shared_load(&slices).is_none());
    }

    #[test]
    fn matching_fat_slices_share_one_load() {
        let one = dyn_image(0x0100_0007, "liba.dylib", "@executable_path/../Frameworks");
        let two = dyn_image(0x0100_000c, "liba.dylib", "@executable_path/../Frameworks");
        let bytes = fat_of(&[(0x0100_0007, 3, one), (0x0100_000c, 0, two)]);
        let image = header::parse(&bytes).unwrap();
        let Image::Fat(fat) = &image else {
            panic!("thin")
        };
        let slices = open_slices(&bytes, fat);
        let shared = shared_load(&slices).expect("same load");
        assert_eq!(shared.kind, Kind::DynamicPie);
        assert_eq!(shared.names(), ["liba.dylib"]);
        assert_eq!(shared.rpath, ["@executable_path/../Frameworks"]);
        assert!(fat_hermetic(&slices).is_none());
        assert_eq!(commands_for(&slices).load, shared);
    }

    #[test]
    fn shared_load_compares_names_not_versions() {
        let mut left = blank(Kind::DynamicPie);
        let mut right = blank(Kind::DynamicPie);
        left.load.needed.push(Dylib {
            cmd: DylibCmd::Load,
            name: "liba.dylib".into(),
            compat: "1.0.0".into(),
            current: "2.0.0".into(),
        });
        right.load.needed.push(Dylib {
            cmd: DylibCmd::Weak,
            name: "liba.dylib".into(),
            compat: "9.0.0".into(),
            current: "9.0.0".into(),
        });
        let slices = [named_slice(left), named_slice(right)];
        let shared = shared_load(&slices).expect("names match");
        assert_eq!(shared.names(), ["liba.dylib"]);
        assert_eq!(shared.needed[0].current, "2.0.0");
    }

    fn named_slice(commands: Commands) -> Slice {
        Slice {
            cputype: 0,
            cpusubtype: 0,
            offset: 0,
            size: 0,
            thin: None,
            commands,
        }
    }

    #[test]
    fn weak_reexport_and_upward_keep_one_row() {
        let mut bytes = header(2, 0, Data::Little);
        push_text(
            &mut bytes,
            LC_LOAD_WEAK_DYLIB | LC_REQ,
            24,
            "weak.dylib",
            Data::Little,
        );
        push_text(
            &mut bytes,
            LC_REEXPORT_DYLIB | LC_REQ,
            24,
            "re.dylib",
            Data::Little,
        );
        push_text(
            &mut bytes,
            LC_LOAD_UPWARD_DYLIB | LC_REQ,
            24,
            "up.dylib",
            Data::Little,
        );
        finish(&mut bytes, Data::Little, 3);
        let image = header::parse(&bytes).unwrap();
        let commands = read(&bytes, &image);
        assert_eq!(
            commands.load.names(),
            ["weak.dylib", "re.dylib", "up.dylib"]
        );
        assert_eq!(
            commands
                .load
                .needed
                .iter()
                .map(|row| row.cmd)
                .collect::<Vec<_>>(),
            [DylibCmd::Weak, DylibCmd::Reexport, DylibCmd::Upward]
        );
    }

    #[test]
    fn a_slice_past_the_end_does_not_panic() {
        let good = dyn_image(0x0100_0007, "liba.dylib", "@executable_path/../Frameworks");
        let mut bytes = fat_of(&[(0x0100_0007, 3, good), (0x0100_000c, 2, vec![0; 1])]);
        bytes[36..40].copy_from_slice(&u32::MAX.to_be_bytes());
        bytes[40..44].copy_from_slice(&u32::MAX.to_be_bytes());
        let image = header::parse(&bytes).unwrap();
        let Image::Fat(fat) = &image else {
            panic!("thin")
        };
        let slices = std::panic::catch_unwind(|| open_slices(&bytes, fat)).unwrap();
        assert_eq!(slices.len(), 2);
        assert!(slices[0].thin.is_some());
        assert_eq!(slices[0].commands.load.names(), ["liba.dylib"]);
        assert!(slices[1].thin.is_none());
        assert_eq!(slices[1].commands.load.kind, Kind::Other);
        assert!(shared_load(&slices).is_none());
        assert_eq!(fat_hermetic(&slices), Some(false));
    }

    #[test]
    fn a_short_window_still_reads_commands_at_the_start() {
        let image = dyn_image(0x0100_0007, "liba.dylib", "@executable_path/../Frameworks");
        let mut bytes = fat_of(&[(0x0100_0007, 3, image.clone())]);
        let lie = (image.len() as u32).saturating_add(50_000_000);
        bytes[20..24].copy_from_slice(&lie.to_be_bytes());
        let parsed = header::parse(&bytes).unwrap();
        let Image::Fat(fat) = &parsed else {
            panic!("thin")
        };
        let slices = open_slices(&bytes, fat);
        assert_eq!(slices[0].commands.load.names(), ["liba.dylib"]);
        assert!(slices[0].size > image.len() as u64);
    }

    #[test]
    fn two_closed_slices_stay_hermetic_when_pie_differs() {
        let mut still = header(2, 0, Data::Little);
        put_u32(&mut still, 4, 0x0100_0007, Data::Little);
        let mut pie = header(2, MH_PIE, Data::Little);
        put_u32(&mut pie, 4, 0x0100_000c, Data::Little);
        let bytes = fat_of(&[(0x0100_0007, 3, still), (0x0100_000c, 0, pie)]);
        let image = header::parse(&bytes).unwrap();
        let Image::Fat(fat) = &image else {
            panic!("thin")
        };
        let slices = open_slices(&bytes, fat);
        assert_eq!(slices[0].commands.load.kind, Kind::Static);
        assert_eq!(slices[1].commands.load.kind, Kind::StaticPie);
        assert!(shared_load(&slices).is_none());
        assert_eq!(fat_hermetic(&slices), Some(true));
    }

    #[test]
    fn fat64_and_a_nested_fat_slice_do_not_panic() {
        let image = dyn_image(0x0100_000c, "liba.dylib", "/opt/lib");
        let mut bytes = vec![0u8; 40];
        bytes[..4].copy_from_slice(&[0xca, 0xfe, 0xba, 0xbf]);
        bytes[4..8].copy_from_slice(&1u32.to_be_bytes());
        bytes[8..12].copy_from_slice(&0x0100_000cu32.to_be_bytes());
        bytes[12..16].copy_from_slice(&2u32.to_be_bytes());
        bytes[16..24].copy_from_slice(&40u64.to_be_bytes());
        bytes[24..32].copy_from_slice(&(image.len() as u64).to_be_bytes());
        bytes.extend_from_slice(&image);
        let parsed = header::parse(&bytes).unwrap();
        let Image::Fat(fat) = &parsed else {
            panic!("thin")
        };
        let slices = open_slices(&bytes, fat);
        assert_eq!(slices.len(), 1);
        assert_eq!(slices[0].cpusubtype, 2);
        assert_eq!(slices[0].commands.load.names(), ["liba.dylib"]);
        assert_eq!(slices[0].commands.load.rpath, ["/opt/lib"]);

        let nested = fat_of(&[(0x0100_0007, 3, image)]);
        let wrapped = fat_of(&[(0x0100_0007, 3, nested)]);
        let parsed = header::parse(&wrapped).unwrap();
        let Image::Fat(fat) = &parsed else {
            panic!("thin")
        };
        let slices = std::panic::catch_unwind(|| open_slices(&wrapped, fat)).unwrap();
        assert!(slices[0].thin.is_none());
        assert_eq!(slices[0].commands.load.kind, Kind::Other);
    }

    #[test]
    fn little_endian_fat_opens_the_slice() {
        let image = dyn_image(0x0100_000c, "liba.dylib", "/opt/lib");
        let mut bytes = vec![0u8; 28];
        bytes[..4].copy_from_slice(&[0xbe, 0xba, 0xfe, 0xca]);
        bytes[4..8].copy_from_slice(&1u32.to_le_bytes());
        bytes[8..12].copy_from_slice(&0x0100_000cu32.to_le_bytes());
        bytes[16..20].copy_from_slice(&28u32.to_le_bytes());
        bytes[20..24].copy_from_slice(&(image.len() as u32).to_le_bytes());
        bytes.extend_from_slice(&image);
        let parsed = header::parse(&bytes).unwrap();
        let Image::Fat(fat) = &parsed else {
            panic!("thin")
        };
        let slices = open_slices(&bytes, fat);
        assert_eq!(slices[0].commands.load.names(), ["liba.dylib"]);
        assert_eq!(slices[0].commands.load.rpath, ["/opt/lib"]);
        assert_eq!(fat.data, Data::Little);
    }

    fn patch_u32(buf: &mut [u8], at: usize, value: u32) {
        put_u32(buf, at, value, Data::Little);
    }

    #[test]
    fn build_version_beats_version_min_and_needed_rows_keep_order() {
        let mut bytes = header(2, 0, Data::Little);
        let build = bytes.len();
        bytes.resize(build + 32, 0);
        patch_u32(&mut bytes, build, LC_BUILD_VERSION);
        patch_u32(&mut bytes, build + 4, 32);
        patch_u32(&mut bytes, build + 8, 1);
        patch_u32(&mut bytes, build + 12, 14 << 16);
        patch_u32(&mut bytes, build + 16, 15 << 16);
        let min = bytes.len();
        bytes.resize(min + 16, 0);
        patch_u32(&mut bytes, min, LC_VERSION_MIN_MACOSX);
        patch_u32(&mut bytes, min + 4, 16);
        patch_u32(&mut bytes, min + 8, 10 << 16);
        patch_u32(&mut bytes, min + 12, 11 << 16);
        let lib_a = bytes.len();
        push_text(&mut bytes, LC_LOAD_DYLIB, 24, "liba.dylib", Data::Little);
        patch_u32(&mut bytes, lib_a + 16, 2 << 16);
        patch_u32(&mut bytes, lib_a + 20, 1 << 16);
        let lib_b = bytes.len();
        push_text(&mut bytes, LC_LOAD_DYLIB, 24, "libb.dylib", Data::Little);
        patch_u32(&mut bytes, lib_b + 16, (4 << 16) | (2 << 8) | 1);
        patch_u32(&mut bytes, lib_b + 20, (3 << 16) | (1 << 8));
        finish(&mut bytes, Data::Little, 4);
        let image = header::parse(&bytes).unwrap();
        let commands = read(&bytes, &image);
        let build = commands.extra.build.expect("build");
        assert_eq!(build.source, "LC_BUILD_VERSION");
        assert_eq!(build.platform, 1);
        assert_eq!(
            (build.minos.major, build.minos.minor, build.minos.patch),
            (14, 0, 0)
        );
        assert_eq!(
            (build.sdk.major, build.sdk.minor, build.sdk.patch),
            (15, 0, 0)
        );
        assert_eq!(commands.load.names(), ["liba.dylib", "libb.dylib"]);
        assert_eq!(commands.load.needed[0].cmd, DylibCmd::Load);
        assert_eq!(commands.load.needed[0].compat, "1.0.0");
        assert_eq!(commands.load.needed[0].current, "2.0.0");
        assert_eq!(commands.load.needed[1].compat, "3.1.0");
        assert_eq!(commands.load.needed[1].current, "4.2.1");
        let text = super::super::page::render("a", &image, &commands, &[], false);
        assert!(text.contains("platform: macos (1)"), "{text}");
        assert!(text.contains("minos: 14.0.0"), "{text}");
        assert!(text.contains("sdk: 15.0.0"), "{text}");
        assert!(text.contains("[LC_BUILD_VERSION]"), "{text}");
        assert!(!text.contains("dylib:"), "{text}");
        let a = text.find("liba.dylib").expect("liba");
        let b = text.find("libb.dylib").expect("libb");
        assert!(a < b, "{text}");
        let between = &text[a..b];
        let compat = between.find("compat: 1.0.0").expect("compat");
        let current = between.find("current: 2.0.0").expect("current");
        assert!(compat < current, "{text}");
        assert!(!between.contains("cmd:"), "{text}");

        let mut only = header(2, 0, Data::Little);
        let at = only.len();
        only.resize(at + 16, 0);
        patch_u32(&mut only, at, LC_VERSION_MIN_MACOSX);
        patch_u32(&mut only, at + 4, 16);
        patch_u32(&mut only, at + 8, 11 << 16);
        patch_u32(&mut only, at + 12, 12 << 16);
        finish(&mut only, Data::Little, 1);
        let image = header::parse(&only).unwrap();
        let commands = read(&only, &image);
        let build = commands.extra.build.expect("version min");
        assert_eq!(build.source, "LC_VERSION_MIN_MACOSX");
        assert_eq!(build.minos.major, 11);
    }

    #[test]
    fn chained_fixups_hide_a_bind_stream() {
        let mut bytes = header(2, 0, Data::Little);
        let chained_at = bytes.len();
        bytes.resize(chained_at + 16, 0);
        patch_u32(&mut bytes, chained_at, LC_DYLD_CHAINED_FIXUPS);
        patch_u32(&mut bytes, chained_at + 4, 16);
        let info_at = bytes.len();
        bytes.resize(info_at + 48, 0);
        patch_u32(&mut bytes, info_at, LC_DYLD_INFO);
        patch_u32(&mut bytes, info_at + 4, 48);
        finish(&mut bytes, Data::Little, 2);

        let blob = bytes.len() as u32;
        let mut body = vec![0u8; 24];
        patch_u32(&mut body, 8, 24);
        patch_u32(&mut body, 12, 28);
        patch_u32(&mut body, 16, 1);
        patch_u32(&mut body, 20, 1);
        body.extend_from_slice(&1u32.to_le_bytes());
        body.extend_from_slice(b"_printf\0");
        patch_u32(&mut bytes, chained_at + 8, blob);
        patch_u32(&mut bytes, chained_at + 12, body.len() as u32);
        bytes.extend_from_slice(&body);

        let bind_off = bytes.len() as u32;
        let mut stream = vec![0x40];
        stream.extend_from_slice(b"_printf\0");
        stream.extend_from_slice(&[0x11, 0x90, 0x00]);
        patch_u32(&mut bytes, info_at + 16, bind_off);
        patch_u32(&mut bytes, info_at + 20, stream.len() as u32);
        bytes.extend_from_slice(&stream);

        let image = header::parse(&bytes).unwrap();
        let rows = read_at(&bytes, &image, true);
        assert_eq!(rows.extra.chained, Some(1));
        assert!(rows.extra.bind.is_none());
        assert_eq!(rows.extra.chained_rows[0].ordinal, 1);
        assert_eq!(rows.extra.chained_rows[0].name, "_printf");
        assert!(!rows.extra.chained_rows[0].weak);
        assert_eq!(rows.extra.uses.get(1).copied(), Some(1));
        let counts = read(&bytes, &image);
        assert_eq!(counts.extra.uses.get(1).copied(), Some(1));
        assert!(counts.extra.chained_rows.is_empty());
        let text = super::super::page::render("a", &image, &counts, &[], false);
        assert!(text.contains("chained_fixups: 1"), "{text}");
        assert!(!text.contains("bind:"), "{text}");
    }

    #[test]
    fn bind_opcodes_print_when_chained_fixups_are_absent() {
        let mut bytes = header(2, 0, Data::Little);
        let info_at = bytes.len();
        bytes.resize(info_at + 48, 0);
        patch_u32(&mut bytes, info_at, LC_DYLD_INFO);
        patch_u32(&mut bytes, info_at + 4, 48);
        finish(&mut bytes, Data::Little, 1);
        let bind_off = bytes.len() as u32;
        let mut stream = vec![0x40];
        stream.extend_from_slice(b"_printf\0");
        stream.extend_from_slice(&[0x11, 0x90, 0x00]);
        patch_u32(&mut bytes, info_at + 16, bind_off);
        patch_u32(&mut bytes, info_at + 20, stream.len() as u32);
        bytes.extend_from_slice(&stream);
        let image = header::parse(&bytes).unwrap();
        let commands = read(&bytes, &image);
        assert!(commands.extra.chained.is_none());
        assert_eq!(commands.extra.bind, Some(1));
        let text = super::super::page::render("a", &image, &commands, &[], false);
        assert!(text.contains("bind: 1"), "{text}");
        assert!(!text.contains("chained_fixups"), "{text}");
    }

    #[test]
    fn an_export_trie_counts_one_terminal() {
        let mut bytes = header(6, 0, Data::Little);
        let cmd = bytes.len();
        bytes.resize(cmd + 16, 0);
        patch_u32(&mut bytes, cmd, LC_DYLD_EXPORTS_TRIE);
        patch_u32(&mut bytes, cmd + 4, 16);
        finish(&mut bytes, Data::Little, 1);
        let off = bytes.len() as u32;
        let trie = [
            0x00, 0x01, b'_', b'f', b'o', b'o', 0x00, 0x08, 0x01, 0x00, 0x00,
        ];
        patch_u32(&mut bytes, cmd + 8, off);
        patch_u32(&mut bytes, cmd + 12, trie.len() as u32);
        bytes.extend_from_slice(&trie);
        let image = header::parse(&bytes).unwrap();
        let commands = read_at(&bytes, &image, true);
        assert_eq!(commands.extra.exports, 1);
        assert_eq!(commands.extra.export_names, ["_foo"]);
        assert_eq!(commands.extra.export_cmd, "LC_DYLD_EXPORTS_TRIE");
    }

    #[test]
    fn a_truncated_command_list_and_an_unmatched_entry_do_not_panic() {
        let mut bytes = header(2, 0, Data::Little);
        patch_u32(&mut bytes, 16, 3);
        patch_u32(&mut bytes, 20, u32::MAX);
        let image = header::parse(&bytes).unwrap();
        assert!(std::panic::catch_unwind(|| read(&bytes, &image)).is_ok());

        let mut bytes = header(2, 0, Data::Little);
        push_main(&mut bytes, 0x1000, Data::Little);
        let cmd = bytes.len();
        bytes.resize(cmd + 24, 0);
        patch_u32(&mut bytes, cmd, LC_SYMTAB);
        patch_u32(&mut bytes, cmd + 4, 24);
        finish(&mut bytes, Data::Little, 2);
        let symoff = bytes.len() as u32;
        bytes.resize(bytes.len() + 16, 0);
        patch_u32(&mut bytes, symoff as usize, 1);
        put_u64(&mut bytes, symoff as usize + 8, 0x9999, Data::Little);
        let stroff = bytes.len() as u32;
        bytes.push(0);
        bytes.extend_from_slice(b"_start\0");
        patch_u32(&mut bytes, cmd + 8, symoff);
        patch_u32(&mut bytes, cmd + 12, 1);
        patch_u32(&mut bytes, cmd + 16, stroff);
        patch_u32(&mut bytes, cmd + 20, 8);
        let image = header::parse(&bytes).unwrap();
        let commands = read(&bytes, &image);
        assert!(commands.entry_is_offset);
        assert!(commands.extra.entry_name.is_none());
        let text = super::super::page::render("a", &image, &commands, &[], false);
        assert!(text.contains("entry_name: null"), "{text}");
    }
}
