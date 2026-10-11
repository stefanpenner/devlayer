/*
---
why: This file reads one Apple disk image trailer.
uses: bininfo
how: bininfo calls this file when the bytes end in a koly trailer.
This file does not inflate a block.
---
*/

//! One UDIF disk image. The `koly` trailer is the record.
//! This module does not inflate a block and does not mount the image.

pub mod page;

const TRAILER: usize = 512;
const NAME_CAP: usize = 32;
const TEXT_CAP: usize = 256;
const PLIST_WINDOW: usize = 1024 * 1024;

/// One Apple disk image. Numbers come from the `koly` trailer.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Image {
    pub variant: [u8; 4],
    pub version: u32,
    pub flags: u32,
    pub sectors: u64,
    pub segment: u32,
    pub segments: u32,
    pub data_offset: u64,
    pub data_length: u64,
    pub xml_offset: u64,
    pub xml_length: u64,
    pub checksum: u32,
    pub names: Vec<String>,
}

/// True when the last 512 bytes are a version-4 `koly` trailer.
pub fn is_dmg(bytes: &[u8]) -> bool {
    let Some(trailer) = trailer(bytes) else {
        return false;
    };
    trailer[..4] == *b"koly" && u32_be(trailer, 4) == Some(4) && u32_be(trailer, 8) == Some(512)
}

/// Read the trailer and the partition names in the resource plist.
/// This function does not inflate a block.
pub fn parse(bytes: &[u8]) -> Result<Image, String> {
    if !is_dmg(bytes) {
        return Err("not a dmg file".into());
    }
    let raw = trailer(bytes).ok_or("file ends before the dmg trailer")?;
    let xml_offset = u64_be(raw, 216).ok_or("file ends before the dmg trailer")?;
    let xml_length = u64_be(raw, 224).ok_or("file ends before the dmg trailer")?;
    let names = plist_names(bytes, xml_offset, xml_length)?;
    let mut variant = [0u8; 4];
    variant.copy_from_slice(
        raw.get(488..492)
            .ok_or("file ends before the dmg trailer")?,
    );
    Ok(Image {
        variant,
        version: need_u32(raw, 4)?,
        flags: need_u32(raw, 12)?,
        sectors: need_u64(raw, 492)?,
        segment: need_u32(raw, 56)?,
        segments: need_u32(raw, 60)?,
        data_offset: need_u64(raw, 24)?,
        data_length: need_u64(raw, 32)?,
        xml_offset,
        xml_length,
        checksum: need_u32(raw, 352)?,
        names,
    })
}

/// Printable fourcc, or the hex word when a byte is not text.
pub fn fourcc(variant: &[u8; 4]) -> String {
    if variant.iter().all(|byte| (0x20..0x7f).contains(byte)) {
        variant.iter().map(|byte| *byte as char).collect()
    } else {
        format!("0x{:X}", u32::from_be_bytes(*variant))
    }
}

/// Image variant word. `None` means the fourcc has no word here.
pub fn variant_name(variant: &[u8; 4]) -> Option<&'static str> {
    Some(match variant {
        b"UDZO" => "zlib",
        b"UDBZ" => "bzip2",
        b"ULFO" => "lzfse",
        b"ULMO" => "lzma",
        b"UDCO" => "adc",
        b"UDRO" => "read-only",
        b"UDRW" => "read-write",
        b"UDTO" => "dvd",
        _ => return None,
    })
}

/// Master checksum word. `None` means the number has no word here.
pub fn checksum_name(kind: u32) -> Option<&'static str> {
    match kind {
        0 => Some("none"),
        2 => Some("crc32"),
        _ => None,
    }
}

fn trailer(bytes: &[u8]) -> Option<&[u8]> {
    if bytes.len() < TRAILER {
        return None;
    }
    bytes.get(bytes.len() - TRAILER..)
}

fn plist_names(bytes: &[u8], offset: u64, length: u64) -> Result<Vec<String>, String> {
    if length == 0 {
        return Ok(Vec::new());
    }
    let start = usize::try_from(offset).map_err(|_| "dmg resource plist is outside this file")?;
    let len = usize::try_from(length).map_err(|_| "dmg resource plist is outside this file")?;
    let end = start
        .checked_add(len)
        .ok_or("dmg resource plist is outside this file")?;
    if end > bytes.len() {
        return Err("dmg resource plist is outside this file".into());
    }
    let window_end = end.min(start.saturating_add(PLIST_WINDOW));
    Ok(scan_names(&bytes[start..window_end]))
}

fn scan_names(bytes: &[u8]) -> Vec<String> {
    let mut names = Vec::new();
    let mut cursor = 0;
    while names.len() < NAME_CAP {
        let name_key = find_sub(bytes, b"<key>Name</key>", cursor);
        let cf_key = find_sub(bytes, b"<key>CFName</key>", cursor);
        let at = match (name_key, cf_key) {
            (Some(left), Some(right)) => left.min(right),
            (Some(left), None) => left,
            (None, Some(right)) => right,
            (None, None) => break,
        };
        let key_len = if bytes[at..].starts_with(b"<key>CFName</key>") {
            17
        } else {
            15
        };
        let rest = at + key_len;
        let Some(string_at) = find_sub(bytes, b"<string>", rest) else {
            break;
        };
        if let Some(next_key) = find_sub(bytes, b"<key>", rest) {
            if next_key < string_at {
                cursor = rest;
                continue;
            }
        }
        let text_at = string_at + b"<string>".len();
        let Some(end) = find_sub(bytes, b"</string>", text_at) else {
            break;
        };
        if end - text_at <= TEXT_CAP {
            if let Ok(text) = std::str::from_utf8(&bytes[text_at..end]) {
                let text = text.trim();
                if !text.is_empty() && !names.iter().any(|name| name == text) {
                    names.push(text.to_string());
                }
            }
        }
        cursor = end + b"</string>".len();
    }
    names
}

fn find_sub(hay: &[u8], needle: &[u8], from: usize) -> Option<usize> {
    if needle.is_empty() || from >= hay.len() {
        return None;
    }
    hay[from..]
        .windows(needle.len())
        .position(|window| window == needle)
        .map(|index| from + index)
}

fn need_u32(bytes: &[u8], at: usize) -> Result<u32, String> {
    u32_be(bytes, at).ok_or_else(|| "file ends before the dmg trailer".into())
}

fn need_u64(bytes: &[u8], at: usize) -> Result<u64, String> {
    u64_be(bytes, at).ok_or_else(|| "file ends before the dmg trailer".into())
}

fn u32_be(bytes: &[u8], at: usize) -> Option<u32> {
    let s = bytes.get(at..at + 4)?;
    Some(u32::from_be_bytes([s[0], s[1], s[2], s[3]]))
}

fn u64_be(bytes: &[u8], at: usize) -> Option<u64> {
    let s = bytes.get(at..at + 8)?;
    Some(u64::from_be_bytes([
        s[0], s[1], s[2], s[3], s[4], s[5], s[6], s[7],
    ]))
}

#[cfg(test)]
fn put_u32(bytes: &mut [u8], at: usize, value: u32) {
    bytes[at..at + 4].copy_from_slice(&value.to_be_bytes());
}

#[cfg(test)]
fn put_u64(bytes: &mut [u8], at: usize, value: u64) {
    bytes[at..at + 8].copy_from_slice(&value.to_be_bytes());
}

#[cfg(test)]
mod tests {
    use super::*;

    fn image(xml: &str) -> Vec<u8> {
        let mut bytes = xml.as_bytes().to_vec();
        let mut koly = vec![0u8; TRAILER];
        koly[..4].copy_from_slice(b"koly");
        put_u32(&mut koly, 4, 4);
        put_u32(&mut koly, 8, 512);
        put_u32(&mut koly, 12, 1);
        put_u64(&mut koly, 24, 512);
        put_u64(&mut koly, 32, 100);
        put_u32(&mut koly, 56, 0);
        put_u32(&mut koly, 60, 1);
        put_u64(&mut koly, 216, 0);
        put_u64(&mut koly, 224, xml.len() as u64);
        put_u32(&mut koly, 352, 2);
        koly[488..492].copy_from_slice(b"UDZO");
        put_u64(&mut koly, 492, 2048);
        bytes.extend_from_slice(&koly);
        bytes
    }

    const PLIST: &str = "\
<?xml version=\"1.0\"?>
<plist><dict><key>resource-fork</key><dict><key>blkx</key><array><dict>\
<key>Name</key><string>whole disk</string>\
<key>CFName</key><string>whole disk (Apple_APFS : 1)</string>\
</dict></array></dict></dict></plist>";

    #[test]
    fn a_koly_trailer_names_the_image() {
        let bytes = image(PLIST);
        assert!(is_dmg(&bytes));
        let image = parse(&bytes).unwrap();
        assert_eq!(fourcc(&image.variant), "UDZO");
        assert_eq!(variant_name(&image.variant), Some("zlib"));
        assert_eq!(image.version, 4);
        assert_eq!(image.flags, 1);
        assert_eq!(image.sectors, 2048);
        assert_eq!(image.segment, 0);
        assert_eq!(image.segments, 1);
        assert_eq!(image.data_offset, 512);
        assert_eq!(image.data_length, 100);
        assert_eq!(image.checksum, 2);
        assert_eq!(image.names, ["whole disk", "whole disk (Apple_APFS : 1)"]);
        let page = page::render("a.dmg", &image, false);
        assert!(page.contains("format: dmg"), "{page}");
        assert!(page.contains("variant: UDZO (zlib)"), "{page}");
        assert!(page.contains("sectors: 2048"), "{page}");
        assert!(page.contains("checksum: crc32 (2)"), "{page}");
        assert!(page.contains("whole disk"), "{page}");
        assert!(!page.contains("\nload:"), "{page}");
        let json = page::render_json("a.dmg", &image);
        assert!(json.contains(r#""format":"dmg""#), "{json}");
        assert!(json.contains(r#""variant_name":"zlib""#), "{json}");
        assert!(!json.contains(r#""kind""#), "{json}");
    }

    #[test]
    fn a_trailer_without_a_plist_has_no_names() {
        let mut koly = vec![0u8; TRAILER];
        koly[..4].copy_from_slice(b"koly");
        put_u32(&mut koly, 4, 4);
        put_u32(&mut koly, 8, 512);
        koly[488..492].copy_from_slice(b"UDRW");
        let image = parse(&koly).unwrap();
        assert!(image.names.is_empty());
        assert_eq!(variant_name(&image.variant), Some("read-write"));
        assert_eq!(image.xml_length, 0);
    }

    #[test]
    fn hostile_dmg_bytes_return_an_error() {
        assert_eq!(parse(&[0; 100]).unwrap_err(), "not a dmg file");
        let mut bad = vec![0u8; TRAILER];
        bad[..4].copy_from_slice(b"koly");
        put_u32(&mut bad, 4, 3);
        put_u32(&mut bad, 8, 512);
        assert!(!is_dmg(&bad));
        assert_eq!(parse(&bad).unwrap_err(), "not a dmg file");
        let mut huge = vec![0u8; TRAILER];
        huge[..4].copy_from_slice(b"koly");
        put_u32(&mut huge, 4, 4);
        put_u32(&mut huge, 8, 512);
        put_u64(&mut huge, 224, u64::MAX);
        assert_eq!(
            parse(&huge).unwrap_err(),
            "dmg resource plist is outside this file"
        );
    }
}
