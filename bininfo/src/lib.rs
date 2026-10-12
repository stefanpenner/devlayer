/*
---
why: This file is the library a caller uses.
uses: bininfo
how: The bininfo command calls open, render, and facts.
Another crate calls the same functions.
---
*/

//! Read one ELF, Mach-O, PE, zip, dmg, or tiff file.
//! `open` reads the bytes. `render` prints the page.
//! `facts` feeds a start claim for ELF, Mach-O, and PE.
//! `zip`, `dmg`, and `tiff` each read one container.
//! An ARW file is a TIFF file.
//! `osload` checks one binary against its policy file.
//! `policy` reads the loader records for one binary.

pub mod claim;
pub mod dmg;
pub mod elf;
pub mod kind;
pub mod macho;
pub mod osload;
pub mod pe;
pub mod policy;
pub mod tiff;
pub mod zip;

mod library;
mod style;

use std::process::Command;

/// One parsed file. Each arm is one format.
pub enum Binary {
    Elf {
        header: elf::header::Header,
        load: elf::load::Load,
        symbols: elf::symbols::Symbols,
    },
    Macho {
        image: macho::header::Image,
        commands: macho::load::Commands,
        slices: Vec<macho::load::Slice>,
    },
    Pe {
        header: pe::header::Header,
        load: pe::load::Load,
        symbols: pe::symbols::Symbols,
    },
    Zip(zip::Archive),
    Dmg(dmg::Image),
    Tiff(tiff::Image),
}

/// Read one file.
/// ELF, Mach-O, and PE win when the prefix matches.
/// `rows` keeps symbol rows for those three.
/// A zip, a dmg, or a tiff ignores `rows`.
pub fn open(bytes: &[u8], rows: bool) -> Result<Binary, String> {
    if elf::header::is_elf(bytes) {
        let header = elf::header::parse(bytes)?;
        let load = elf::load::read(bytes, &header);
        let symbols = elf::symbols::read(bytes, &header, rows);
        return Ok(Binary::Elf {
            header,
            load,
            symbols,
        });
    }
    if macho::header::is_macho(bytes) {
        let image = macho::header::parse(bytes)?;
        let slices = macho::load::slices_rows(bytes, &image, rows);
        let commands = match &image {
            macho::header::Image::Fat(_) => macho::load::commands_for(&slices),
            macho::header::Image::Thin(_) => macho::load::read_at(bytes, &image, rows),
        };
        return Ok(Binary::Macho {
            image,
            commands,
            slices,
        });
    }
    if pe::header::is_pe(bytes) {
        let header = pe::header::parse(bytes)?;
        let load = pe::load::read(bytes, &header);
        let symbols = pe::symbols::read(bytes, &header, rows);
        return Ok(Binary::Pe {
            header,
            load,
            symbols,
        });
    }
    if zip::is_zip(bytes) {
        return Ok(Binary::Zip(zip::parse(bytes)?));
    }
    if dmg::is_dmg(bytes) {
        return Ok(Binary::Dmg(dmg::parse(bytes)?));
    }
    if tiff::is_tiff(bytes) {
        return Ok(Binary::Tiff(tiff::parse(bytes)?));
    }
    Err("not an ELF, Mach-O, PE, zip, dmg, or tiff file".into())
}

/// Print one page, or one JSON object when `json` is true.
pub fn render(path: &str, file: &Binary, json: bool, color: bool) -> String {
    match file {
        Binary::Elf {
            header,
            load,
            symbols,
        } => {
            if json {
                elf::json::render_with(path, header, load, symbols)
            } else {
                elf::page::render_with(path, header, load, symbols, color)
            }
        }
        Binary::Macho {
            image,
            commands,
            slices,
        } => {
            if json {
                macho::json::render(path, image, commands, slices)
            } else {
                macho::page::render(path, image, commands, slices, color)
            }
        }
        Binary::Pe {
            header,
            load,
            symbols,
        } => {
            if json {
                pe::json::render_with(path, header, load, symbols)
            } else {
                pe::page::render_with(path, header, load, symbols, color)
            }
        }
        Binary::Zip(archive) => {
            if json {
                zip::page::render_json(path, archive)
            } else {
                zip::page::render(path, archive, color)
            }
        }
        Binary::Dmg(image) => {
            if json {
                dmg::page::render_json(path, image)
            } else {
                dmg::page::render(path, image, color)
            }
        }
        Binary::Tiff(image) => {
            if json {
                tiff::page::render_json(path, image)
            } else {
                tiff::page::render(path, image, color)
            }
        }
    }
}

/// Build the facts a claim checks. `host` does not change the hermetic result.
/// A zip, a dmg, or a tiff has no start claim.
pub fn facts(path: &str, file: &Binary, host: &claim::Host) -> Result<claim::Facts, String> {
    match file {
        Binary::Elf {
            header,
            load,
            symbols,
        } => Ok(claim::Facts {
            file: path.to_string(),
            format: "elf",
            kind: load.kind,
            interp: load.interp.clone(),
            needed: load.needed.clone(),
            pie: load.pie,
            fat_hermetic: None,
            host: claim::host_marks(&elf::symbols::floors(header, symbols), host),
        }),
        Binary::Macho {
            image,
            commands,
            slices,
        } => Ok(claim::Facts {
            file: path.to_string(),
            format: "macho",
            kind: commands.load.kind,
            interp: commands.load.interp.clone(),
            needed: commands.load.names(),
            pie: commands.load.pie,
            fat_hermetic: macho::load::fat_hermetic(slices),
            host: claim::host_marks(&macho::symbols::floors(image, commands, slices), host),
        }),
        Binary::Pe {
            header,
            load,
            symbols,
        } => Ok(claim::Facts {
            file: path.to_string(),
            format: "pe",
            kind: load.kind,
            interp: load.interp.clone(),
            needed: load.needed.clone(),
            pie: load.pie,
            fat_hermetic: None,
            host: claim::host_marks(&pe::symbols::floors(header, symbols), host),
        }),
        Binary::Zip(_) => Err(no_claim("zip")),
        Binary::Dmg(_) => Err(no_claim("dmg")),
        Binary::Tiff(image) => Err(no_claim(image.profile.name())),
    }
}

fn no_claim(format: &str) -> String {
    let article = if format == "arw" { "an" } else { "a" };
    format!("{article} {format} file has no start claim")
}

/// Read the CPU and OS of this machine.
pub fn current_host() -> claim::Host {
    let arch = std::env::consts::ARCH;
    let os = std::env::consts::OS;
    let version = match os {
        "macos" => command_version("sw_vers", &["-productVersion"]),
        "linux" => command_version("uname", &["-r"]),
        _ => None,
    };
    claim::Host {
        arch,
        os,
        version,
        pac: os == "macos" && arch == "aarch64",
    }
}

fn command_version(bin: &str, args: &[&str]) -> Option<claim::Version> {
    let out = Command::new(bin).args(args).output().ok()?;
    if !out.status.success() {
        return None;
    }
    let text = String::from_utf8(out.stdout).ok()?;
    claim::parse_version(text.trim())
}
