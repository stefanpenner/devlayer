/*
---
why: This file checks that another crate can call the library.
uses: cargo test
how: cargo test builds this file against the bininfo library.
The test does not run the bininfo command.
---
*/

use bininfo::claim::Host;
use bininfo::kind::{self, Kind};

fn host() -> Host {
    Host {
        arch: "aarch64",
        os: "macos",
        version: None,
        pac: true,
    }
}

fn static_elf() -> Vec<u8> {
    let mut bytes = vec![0u8; 64];
    bytes[0..4].copy_from_slice(&[0x7f, b'E', b'L', b'F']);
    bytes[4] = 2;
    bytes[5] = 1;
    bytes[6] = 1;
    bytes[16] = 2;
    bytes[18] = 62;
    bytes
}

#[test]
fn a_caller_opens_one_elf_and_checks_a_claim() {
    let bytes = static_elf();
    assert!(bininfo::elf::header::is_elf(&bytes));
    assert!(!bininfo::macho::header::is_macho(&bytes));
    assert!(!bininfo::pe::header::is_pe(&bytes));

    let file = bininfo::open(&bytes, false).unwrap();
    match &file {
        bininfo::Binary::Elf { load, .. } => assert_eq!(load.kind, Kind::Static),
        _ => panic!("expected an ELF file"),
    }
    let page = bininfo::render("tiny", &file, false, false);
    assert!(page.contains("kind: static"), "{page}");
    assert!(page.contains("format: elf"), "{page}");

    let json = bininfo::render("tiny", &file, true, false);
    assert!(json.contains(r#""kind":"static""#), "{json}");

    let facts = bininfo::facts("tiny", &file, &host()).unwrap();
    assert_eq!(facts.kind, Kind::Static);
    assert_eq!(kind::name(facts.kind), "static");
    let claims = bininfo::claim::parse(&["hermetic".to_string()]).unwrap();
    let misses = bininfo::claim::judge(&facts, &claims);
    assert!(misses.is_empty(), "{misses:?}");
}

#[test]
fn a_caller_reads_zip_dmg_and_tiff_as_their_own_files() {
    let zip_bytes = store_zip(&[("META-INF/MANIFEST.MF", b"A")]);
    let zip = bininfo::open(&zip_bytes, false).unwrap();
    match &zip {
        bininfo::Binary::Zip(archive) => {
            assert_eq!(archive.profile, "jar");
            assert_eq!(archive.members[0].name, "META-INF/MANIFEST.MF");
        }
        _ => panic!("expected a zip file"),
    }
    let page = bininfo::render("a.jar", &zip, false, false);
    assert!(page.contains("format: zip"), "{page}");
    assert!(page.contains("profile: jar"), "{page}");
    assert!(!page.contains("\nload:"), "{page}");
    assert_eq!(
        bininfo::facts("a.jar", &zip, &host()).unwrap_err(),
        "a zip file has no start claim"
    );
    assert_eq!(bininfo::zip::parse(&zip_bytes).unwrap().profile, "jar");

    let dmg_bytes = koly();
    let dmg = bininfo::open(&dmg_bytes, false).unwrap();
    match &dmg {
        bininfo::Binary::Dmg(image) => assert_eq!(bininfo::dmg::fourcc(&image.variant), "UDZO"),
        _ => panic!("expected a dmg file"),
    }
    let page = bininfo::render("a.dmg", &dmg, false, false);
    assert!(page.contains("format: dmg"), "{page}");
    assert!(!page.contains("\nload:"), "{page}");
    assert_eq!(
        bininfo::facts("a.dmg", &dmg, &host()).unwrap_err(),
        "a dmg file has no start claim"
    );

    let arw_bytes = sony_tiff();
    let arw = bininfo::open(&arw_bytes, false).unwrap();
    match &arw {
        bininfo::Binary::Tiff(image) => {
            assert_eq!(image.profile, bininfo::tiff::Profile::Arw);
            assert_eq!(image.make.as_deref(), Some("SONY"));
        }
        _ => panic!("expected a tiff file"),
    }
    let page = bininfo::render("a.ARW", &arw, false, false);
    assert!(page.contains("format: arw"), "{page}");
    assert!(!page.contains("\nload:"), "{page}");
    assert_eq!(
        bininfo::facts("a.ARW", &arw, &host()).unwrap_err(),
        "an arw file has no start claim"
    );
    assert!(bininfo::tiff::parse(&arw_bytes).is_ok());

    let mut elf = static_elf();
    elf.resize(512, 0);
    elf.extend_from_slice(&koly());
    let file = bininfo::open(&elf, false).unwrap();
    assert!(matches!(file, bininfo::Binary::Elf { .. }));
}

fn store_zip(files: &[(&str, &[u8])]) -> Vec<u8> {
    let mut local = Vec::new();
    let mut central = Vec::new();
    for (name, data) in files {
        let offset = u32::try_from(local.len()).unwrap();
        local.extend_from_slice(&0x0403_4b50u32.to_le_bytes());
        local.extend_from_slice(&20u16.to_le_bytes());
        local.extend_from_slice(&[0; 8]);
        local.extend_from_slice(&(data.len() as u32).to_le_bytes());
        local.extend_from_slice(&(data.len() as u32).to_le_bytes());
        local.extend_from_slice(&(name.len() as u16).to_le_bytes());
        local.extend_from_slice(&0u16.to_le_bytes());
        local.extend_from_slice(name.as_bytes());
        local.extend_from_slice(data);
        central.extend_from_slice(&0x0201_4b50u32.to_le_bytes());
        central.extend_from_slice(&20u16.to_le_bytes());
        central.extend_from_slice(&20u16.to_le_bytes());
        central.extend_from_slice(&[0; 8]);
        central.extend_from_slice(&0u32.to_le_bytes());
        central.extend_from_slice(&(data.len() as u32).to_le_bytes());
        central.extend_from_slice(&(data.len() as u32).to_le_bytes());
        central.extend_from_slice(&(name.len() as u16).to_le_bytes());
        central.extend_from_slice(&[0; 12]);
        central.extend_from_slice(&offset.to_le_bytes());
        central.extend_from_slice(name.as_bytes());
    }
    let cd_offset = u32::try_from(local.len()).unwrap();
    let cd_size = u32::try_from(central.len()).unwrap();
    local.extend(central);
    local.extend_from_slice(&0x0605_4b50u32.to_le_bytes());
    local.extend_from_slice(&0u16.to_le_bytes());
    local.extend_from_slice(&0u16.to_le_bytes());
    local.extend_from_slice(&(files.len() as u16).to_le_bytes());
    local.extend_from_slice(&(files.len() as u16).to_le_bytes());
    local.extend_from_slice(&cd_size.to_le_bytes());
    local.extend_from_slice(&cd_offset.to_le_bytes());
    local.extend_from_slice(&0u16.to_le_bytes());
    local
}

fn koly() -> Vec<u8> {
    let mut bytes = vec![0u8; 512];
    bytes[..4].copy_from_slice(b"koly");
    bytes[4..8].copy_from_slice(&4u32.to_be_bytes());
    bytes[8..12].copy_from_slice(&512u32.to_be_bytes());
    bytes[488..492].copy_from_slice(b"UDZO");
    bytes
}

fn sony_tiff() -> Vec<u8> {
    let mut bytes = vec![0u8; 38];
    bytes[..4].copy_from_slice(b"II*\0");
    bytes[4] = 8;
    bytes[8] = 2;
    bytes[10..12].copy_from_slice(&271u16.to_le_bytes());
    bytes[12..14].copy_from_slice(&2u16.to_le_bytes());
    bytes[14..18].copy_from_slice(&4u32.to_le_bytes());
    bytes[18..22].copy_from_slice(b"SONY");
    bytes[22..24].copy_from_slice(&256u16.to_le_bytes());
    bytes[24..26].copy_from_slice(&3u16.to_le_bytes());
    bytes[26..30].copy_from_slice(&1u32.to_le_bytes());
    bytes[30] = 16;
    bytes
}
