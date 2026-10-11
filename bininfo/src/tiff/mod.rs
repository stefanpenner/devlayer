/*
---
why: This file reads one TIFF header and the IFD tags.
uses: bininfo
how: bininfo calls this file when the bytes are a TIFF file.
A Sony ARW file is a TIFF file.
This file does not decode the sensor bytes.
---
*/

//! One TIFF file. An ARW file is a TIFF file whose make is SONY.
//! Compression 32767 is also ARW. This module does not decode sensor bytes.

pub mod page;

const IFD_CAP: usize = 32;
const ENTRY_CAP: usize = 4096;
const TEXT_CAP: usize = 256;
const SUB_CAP: usize = 8;

const MAKE: u16 = 271;
const MODEL: u16 = 272;
const WIDTH: u16 = 256;
const HEIGHT: u16 = 257;
const BITS: u16 = 258;
const COMPRESSION: u16 = 259;
const STRIP: u16 = 273;
const SUBIFD: u16 = 330;
const JPEG: u16 = 513;

/// Byte order of the TIFF file.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Order {
    Little,
    Big,
}

impl Order {
    pub fn name(self) -> &'static str {
        match self {
            Self::Little => "little",
            Self::Big => "big",
        }
    }
}

/// `arw` is a Sony TIFF. `tiff` is any other TIFF.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Profile {
    Arw,
    Tiff,
}

impl Profile {
    pub fn name(self) -> &'static str {
        match self {
            Self::Arw => "arw",
            Self::Tiff => "tiff",
        }
    }
}

/// One IFD. `role` is `ifd` or `subifd`.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Ifd {
    pub offset: u64,
    pub entries: u64,
    pub next: u64,
    pub role: &'static str,
}

/// One TIFF image. Width and height come from the chosen frame.
/// The chosen frame is the ARW compression frame, or the widest frame.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Image {
    pub order: Order,
    pub profile: Profile,
    pub make: Option<String>,
    pub model: Option<String>,
    pub compression: Option<u16>,
    pub width: Option<u64>,
    pub height: Option<u64>,
    pub bits: Option<u64>,
    pub ifd0: u64,
    pub strip: Option<u64>,
    pub jpeg: Option<u64>,
    pub ifds: Vec<Ifd>,
    pub more: bool,
}

struct Frame {
    ifd: Ifd,
    make: Option<String>,
    model: Option<String>,
    compression: Option<u16>,
    width: Option<u64>,
    height: Option<u64>,
    bits: Option<u64>,
    strip: Option<u64>,
    jpeg: Option<u64>,
}

/// True when the bytes start with a classic TIFF header.
pub fn is_tiff(bytes: &[u8]) -> bool {
    header(bytes).is_some()
}

/// Read the header and the IFD tags. This function does not decode sensor bytes.
pub fn parse(bytes: &[u8]) -> Result<Image, String> {
    let (order, ifd0) = header(bytes).ok_or("not a tiff file")?;
    let (frames, more) = walk(bytes, order, ifd0)?;
    if frames.is_empty() {
        return Err("tiff ifd is outside this file".into());
    }
    let chosen = choose(&frames);
    let make = frames.iter().find_map(|frame| frame.make.clone());
    let model = frames.iter().find_map(|frame| frame.model.clone());
    let profile = if make
        .as_deref()
        .is_some_and(|text| text.eq_ignore_ascii_case("SONY"))
        || frames.iter().any(|frame| frame.compression == Some(32767))
    {
        Profile::Arw
    } else {
        Profile::Tiff
    };
    Ok(Image {
        order,
        profile,
        make,
        model,
        compression: chosen.compression,
        width: chosen.width,
        height: chosen.height,
        bits: chosen.bits,
        ifd0,
        strip: chosen.strip,
        jpeg: chosen.jpeg,
        ifds: frames.into_iter().map(|frame| frame.ifd).collect(),
        more,
    })
}

/// Compression word. `None` means the number has no word here.
pub fn compression_name(value: u16) -> Option<&'static str> {
    Some(match value {
        1 => "none",
        5 => "lzw",
        6 | 7 => "jpeg",
        8 => "deflate",
        32767 => "sony-arw",
        32773 => "packbits",
        _ => return None,
    })
}

fn header(bytes: &[u8]) -> Option<(Order, u64)> {
    if bytes.len() < 8 {
        return None;
    }
    let order = if bytes.starts_with(b"II*\0") {
        Order::Little
    } else if bytes.starts_with(b"MM\0*") {
        Order::Big
    } else {
        return None;
    };
    let off = u64::from(u32_at(bytes, 4, order)?);
    if off < 8 {
        return None;
    }
    let at = usize::try_from(off).ok()?;
    if at.checked_add(2)? > bytes.len() {
        return None;
    }
    Some((order, off))
}

fn walk(bytes: &[u8], order: Order, ifd0: u64) -> Result<(Vec<Frame>, bool), String> {
    let mut queue = vec![(ifd0, "ifd")];
    let mut seen = Vec::new();
    let mut frames = Vec::new();
    let mut index = 0;
    let mut more = false;
    while index < queue.len() {
        if frames.len() == IFD_CAP {
            more = true;
            break;
        }
        let (offset, role) = queue[index];
        index += 1;
        if seen.contains(&offset) {
            continue;
        }
        seen.push(offset);
        let (frame, subs) = read_ifd(bytes, order, offset, role)?;
        for sub in subs {
            if sub != 0 {
                queue.push((sub, "subifd"));
            }
        }
        if frame.ifd.next != 0 {
            queue.push((frame.ifd.next, role));
        }
        frames.push(frame);
        if queue.len() > IFD_CAP * 4 {
            more = true;
            break;
        }
    }
    Ok((frames, more))
}

fn read_ifd(
    bytes: &[u8],
    order: Order,
    offset: u64,
    role: &'static str,
) -> Result<(Frame, Vec<u64>), String> {
    let at = usize::try_from(offset).map_err(|_| "tiff ifd is outside this file")?;
    let count = u16_at(bytes, at, order).ok_or("tiff ifd is outside this file")? as usize;
    if count > ENTRY_CAP {
        return Err("tiff ifd has too many entries".into());
    }
    let body = count
        .checked_mul(12)
        .and_then(|body| body.checked_add(6))
        .ok_or("tiff ifd is outside this file")?;
    let end = at
        .checked_add(body)
        .ok_or("tiff ifd is outside this file")?;
    if end > bytes.len() {
        return Err("tiff ifd is outside this file".into());
    }
    let mut frame = Frame {
        ifd: Ifd {
            offset,
            entries: count as u64,
            next: 0,
            role,
        },
        make: None,
        model: None,
        compression: None,
        width: None,
        height: None,
        bits: None,
        strip: None,
        jpeg: None,
    };
    let mut subs = Vec::new();
    for index in 0..count {
        let entry = at + 2 + index * 12;
        let tag = u16_at(bytes, entry, order).unwrap_or(0);
        match tag {
            MAKE => frame.make = ascii(bytes, order, entry),
            MODEL => frame.model = ascii(bytes, order, entry),
            WIDTH => frame.width = first_int(bytes, order, entry),
            HEIGHT => frame.height = first_int(bytes, order, entry),
            BITS => frame.bits = first_int(bytes, order, entry),
            COMPRESSION => {
                frame.compression =
                    first_int(bytes, order, entry).and_then(|value| u16::try_from(value).ok())
            }
            STRIP => frame.strip = first_int(bytes, order, entry),
            JPEG => frame.jpeg = first_int(bytes, order, entry),
            SUBIFD => push_subs(bytes, order, entry, &mut subs),
            _ => {}
        }
    }
    let next_at = at + 2 + count * 12;
    frame.ifd.next = u64::from(u32_at(bytes, next_at, order).unwrap_or(0));
    Ok((frame, subs))
}

fn push_subs(bytes: &[u8], order: Order, entry: usize, subs: &mut Vec<u64>) {
    let Some(kind) = u16_at(bytes, entry + 2, order) else {
        return;
    };
    if kind != 3 && kind != 4 && kind != 13 {
        return;
    }
    let Some(count) = u32_at(bytes, entry + 4, order) else {
        return;
    };
    let size = if kind == 3 { 2 } else { 4 };
    let total = (count as usize).saturating_mul(size);
    let keep = (count as usize).min(SUB_CAP);
    if total <= 4 {
        for index in 0..keep {
            let at = entry + 8 + index * size;
            if let Some(value) = int_at(bytes, order, at, size) {
                subs.push(value);
            }
        }
        return;
    }
    let Some(off) = u32_at(bytes, entry + 8, order) else {
        return;
    };
    for index in 0..keep {
        let at = off as usize + index * size;
        if let Some(value) = int_at(bytes, order, at, size) {
            subs.push(value);
        }
    }
}

fn choose(frames: &[Frame]) -> &Frame {
    if let Some(frame) = frames.iter().find(|frame| frame.compression == Some(32767)) {
        return frame;
    }
    frames
        .iter()
        .max_by_key(|frame| frame.width.unwrap_or(0))
        .unwrap_or(&frames[0])
}

fn ascii(bytes: &[u8], order: Order, entry: usize) -> Option<String> {
    if u16_at(bytes, entry + 2, order)? != 2 {
        return None;
    }
    let count = u32_at(bytes, entry + 4, order)? as usize;
    if count == 0 {
        return None;
    }
    let raw = if count <= 4 {
        bytes.get(entry + 8..entry + 8 + count.min(4))?
    } else {
        let off = u32_at(bytes, entry + 8, order)? as usize;
        let n = count.min(TEXT_CAP);
        bytes.get(off..off + n)?
    };
    let end = raw.iter().position(|byte| *byte == 0).unwrap_or(raw.len());
    let text = String::from_utf8_lossy(&raw[..end]);
    let text = text.trim();
    if text.is_empty() {
        None
    } else {
        Some(text.to_string())
    }
}

fn first_int(bytes: &[u8], order: Order, entry: usize) -> Option<u64> {
    let kind = u16_at(bytes, entry + 2, order)?;
    let count = u32_at(bytes, entry + 4, order)?;
    if count == 0 {
        return None;
    }
    let size = match kind {
        3 => 2,
        4 => 4,
        _ => return None,
    };
    let total = (count as usize).checked_mul(size)?;
    if total <= 4 {
        return int_at(bytes, order, entry + 8, size);
    }
    let off = u32_at(bytes, entry + 8, order)? as usize;
    int_at(bytes, order, off, size)
}

fn int_at(bytes: &[u8], order: Order, at: usize, size: usize) -> Option<u64> {
    let raw = bytes.get(at..at + size)?;
    Some(match (order, size) {
        (Order::Little, 2) => u64::from(u16::from_le_bytes([raw[0], raw[1]])),
        (Order::Big, 2) => u64::from(u16::from_be_bytes([raw[0], raw[1]])),
        (Order::Little, 4) => u64::from(u32::from_le_bytes([raw[0], raw[1], raw[2], raw[3]])),
        (Order::Big, 4) => u64::from(u32::from_be_bytes([raw[0], raw[1], raw[2], raw[3]])),
        _ => return None,
    })
}

fn u16_at(bytes: &[u8], at: usize, order: Order) -> Option<u16> {
    int_at(bytes, order, at, 2).map(|value| value as u16)
}

fn u32_at(bytes: &[u8], at: usize, order: Order) -> Option<u32> {
    int_at(bytes, order, at, 4).map(|value| value as u32)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn sony_arw() -> Vec<u8> {
        let mut bytes = vec![0u8; 92];
        bytes[..4].copy_from_slice(b"II*\0");
        bytes[4] = 8;
        bytes[8] = 3;
        put_entry(&mut bytes, 10, 271, 2, 4, u32::from_le_bytes(*b"SONY"));
        put_entry(&mut bytes, 22, 256, 3, 1, 160);
        put_entry(&mut bytes, 34, 330, 4, 1, 50);
        put_entry(&mut bytes, 52, 256, 4, 1, 7008);
        put_entry(&mut bytes, 64, 257, 4, 1, 4672);
        put_entry(&mut bytes, 76, 259, 3, 1, 32767);
        bytes[50] = 3;
        bytes
    }

    fn put_entry(bytes: &mut [u8], at: usize, tag: u16, kind: u16, count: u32, value: u32) {
        bytes[at..at + 2].copy_from_slice(&tag.to_le_bytes());
        bytes[at + 2..at + 4].copy_from_slice(&kind.to_le_bytes());
        bytes[at + 4..at + 8].copy_from_slice(&count.to_le_bytes());
        bytes[at + 8..at + 12].copy_from_slice(&value.to_le_bytes());
    }

    #[test]
    fn a_sony_subifd_is_an_arw_frame() {
        let bytes = sony_arw();
        assert!(is_tiff(&bytes));
        let image = parse(&bytes).unwrap();
        assert_eq!(image.profile, Profile::Arw);
        assert_eq!(image.order, Order::Little);
        assert_eq!(image.make.as_deref(), Some("SONY"));
        assert_eq!(image.width, Some(7008));
        assert_eq!(image.height, Some(4672));
        assert_eq!(image.compression, Some(32767));
        assert_eq!(image.ifds.len(), 2);
        assert_eq!(image.ifds[0].role, "ifd");
        assert_eq!(image.ifds[1].role, "subifd");
        assert_eq!(image.ifds[1].offset, 50);
        let page = page::render("a.ARW", &image, false);
        assert!(page.contains("format: arw"), "{page}");
        assert!(page.contains("make: SONY"), "{page}");
        assert!(page.contains("compression: sony-arw (32767)"), "{page}");
        assert!(page.contains("width: 7008"), "{page}");
        assert!(page.contains("role: subifd"), "{page}");
        assert!(!page.contains("\nload:"), "{page}");
        let json = page::render_json("a.ARW", &image);
        assert!(json.contains(r#""format":"arw""#), "{json}");
        assert!(json.contains(r#""width":7008"#), "{json}");
        assert!(!json.contains(r#""kind""#), "{json}");
    }

    #[test]
    fn a_big_endian_tiff_stays_tiff() {
        let mut bytes = vec![0u8; 26];
        bytes[..4].copy_from_slice(b"MM\0*");
        bytes[7] = 8;
        bytes[9] = 1;
        bytes[10..12].copy_from_slice(&256u16.to_be_bytes());
        bytes[12..14].copy_from_slice(&3u16.to_be_bytes());
        bytes[17] = 1;
        bytes[18] = 0;
        bytes[19] = 32;
        let image = parse(&bytes).unwrap();
        assert_eq!(image.profile, Profile::Tiff);
        assert_eq!(image.order, Order::Big);
        assert_eq!(image.make, None);
        assert_eq!(image.width, Some(32));
        let page = page::render("a.tif", &image, false);
        assert!(page.contains("format: tiff"), "{page}");
        assert!(page.contains("order: big"), "{page}");
        assert!(page.contains("make: null"), "{page}");
    }

    #[test]
    fn an_ifd_loop_stops() {
        let mut bytes = vec![0u8; 26];
        bytes[..4].copy_from_slice(b"II*\0");
        bytes[4] = 8;
        bytes[8] = 1;
        put_entry(&mut bytes, 10, 256, 3, 1, 1);
        bytes[22] = 8;
        let image = parse(&bytes).unwrap();
        assert_eq!(image.ifds.len(), 1);
        assert!(!image.more);
    }

    #[test]
    fn hostile_tiff_bytes_return_an_error() {
        assert_eq!(parse(b"").unwrap_err(), "not a tiff file");
        assert_eq!(parse(b"II*\0").unwrap_err(), "not a tiff file");
        assert!(parse(b"II*\0\x08\x00\x00\x00").is_err());
        let mut huge = vec![0u8; 16];
        huge[..4].copy_from_slice(b"II*\0");
        huge[4] = 8;
        huge[8] = 0x88;
        huge[9] = 0x13;
        assert_eq!(parse(&huge).unwrap_err(), "tiff ifd has too many entries");
        let mut junk = vec![0u8; 4096];
        junk[..4].copy_from_slice(b"II*\0");
        let _ = parse(&junk);
    }
}
