/*
---
why: This file reads one zip central directory.
uses: bininfo
how: bininfo calls this file when the bytes are a zip.
This file does not inflate a member.
---
*/

//! One zip archive. The central directory is the index.
//! This module does not inflate a member.

pub mod page;

const MEMBER_CAP: usize = 4096;
const NAME_CAP: usize = 4096;
const CD_SIG: [u8; 4] = [0x50, 0x4b, 0x01, 0x02];
const EOCD_SIG: [u8; 4] = [0x50, 0x4b, 0x05, 0x06];
const ZIP64_LOC_SIG: [u8; 4] = [0x50, 0x4b, 0x06, 0x07];
const ZIP64_EOCD_SIG: [u8; 4] = [0x50, 0x4b, 0x06, 0x06];

/// One stored member. `offset` is the local header.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Member {
    pub name: String,
    pub method: u16,
    pub size: u64,
    pub compressed: u64,
    pub crc: u32,
    pub offset: u64,
    pub encrypted: bool,
}

/// One zip archive. `count` is the central-directory count.
/// `listed` is the number of rows this record keeps.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Archive {
    pub zip64: bool,
    pub spanned: bool,
    pub encrypted: bool,
    pub count: u64,
    pub listed: usize,
    pub comment: u16,
    pub profile: &'static str,
    pub members: Vec<Member>,
}

struct Eocd {
    at: usize,
    disk: u16,
    cd_disk: u16,
    entries: u16,
    cd_size: u32,
    cd_offset: u32,
    comment: u16,
}

struct Dir {
    offset: u64,
    size: u64,
    count: u64,
    zip64: bool,
    spanned: bool,
}

/// True when the bytes look like a zip.
/// A `PK` prefix is enough. A tail match also needs a real directory.
pub fn is_zip(bytes: &[u8]) -> bool {
    if prefix_zip(bytes) {
        return true;
    }
    let Some(eocd) = find_eocd(bytes) else {
        return false;
    };
    match directory(bytes, &eocd) {
        Ok(dir) if dir.count == 0 && dir.size == 0 => true,
        Ok(dir) => has_cd_sig(bytes, dir.offset),
        Err(_) => false,
    }
}

/// Read the central directory. This function does not inflate a member.
pub fn parse(bytes: &[u8]) -> Result<Archive, String> {
    if !is_zip(bytes) {
        return Err("not a zip file".into());
    }
    let eocd = find_eocd(bytes).ok_or("file ends before the zip end record")?;
    let dir = directory(bytes, &eocd)?;
    let members = read_members(bytes, &dir)?;
    let profile = profile_of(&members);
    let encrypted = members.iter().any(|row| row.encrypted);
    let listed = members.len();
    Ok(Archive {
        zip64: dir.zip64,
        spanned: dir.spanned,
        encrypted,
        count: dir.count,
        listed,
        comment: eocd.comment,
        profile,
        members,
    })
}

/// Compression method word. `None` means the number has no word here.
pub fn method_name(method: u16) -> Option<&'static str> {
    Some(match method {
        0 => "store",
        8 => "deflate",
        9 => "deflate64",
        12 => "bzip2",
        14 => "lzma",
        93 => "zstd",
        95 => "xz",
        _ => return None,
    })
}

fn prefix_zip(bytes: &[u8]) -> bool {
    matches!(
        bytes.get(..4),
        Some(
            [0x50, 0x4b, 0x03, 0x04]
                | [0x50, 0x4b, 0x05, 0x06]
                | [0x50, 0x4b, 0x06, 0x06]
                | [0x50, 0x4b, 0x07, 0x08]
        )
    )
}

fn find_eocd(bytes: &[u8]) -> Option<Eocd> {
    if bytes.len() < 22 {
        return None;
    }
    let earliest = bytes.len() - 22 - bytes.len().saturating_sub(22).min(65_535);
    let mut at = bytes.len() - 22;
    loop {
        if bytes.get(at..at + 4) == Some(&EOCD_SIG) {
            let comment = u16_le(bytes, at + 20)?;
            let tail = at
                .checked_add(22)
                .and_then(|end| end.checked_add(comment as usize));
            if tail == Some(bytes.len()) {
                return Some(Eocd {
                    at,
                    disk: u16_le(bytes, at + 4)?,
                    cd_disk: u16_le(bytes, at + 6)?,
                    entries: u16_le(bytes, at + 10)?,
                    cd_size: u32_le(bytes, at + 12)?,
                    cd_offset: u32_le(bytes, at + 16)?,
                    comment,
                });
            }
        }
        if at == earliest {
            break;
        }
        at -= 1;
    }
    None
}

fn directory(bytes: &[u8], eocd: &Eocd) -> Result<Dir, String> {
    let spanned = eocd.disk != 0 || eocd.cd_disk != 0;
    let zip64 =
        eocd.entries == 0xffff || eocd.cd_size == 0xffff_ffff || eocd.cd_offset == 0xffff_ffff;
    if !zip64 {
        return Ok(Dir {
            offset: u64::from(eocd.cd_offset),
            size: u64::from(eocd.cd_size),
            count: u64::from(eocd.entries),
            zip64: false,
            spanned,
        });
    }
    if eocd.at < 20 {
        return Err("zip64 locator is outside this file".into());
    }
    let loc = eocd.at - 20;
    if bytes.get(loc..loc + 4) != Some(&ZIP64_LOC_SIG) {
        return Err("zip64 locator signature is wrong".into());
    }
    let zoff = u64_le(bytes, loc + 8).ok_or("zip64 locator is outside this file")?;
    let z = usize::try_from(zoff).map_err(|_| "zip64 end record is outside this file")?;
    if bytes.get(z..z + 4) != Some(&ZIP64_EOCD_SIG) {
        return Err("zip64 end record signature is wrong".into());
    }
    if bytes.len().saturating_sub(z) < 56 {
        return Err("zip64 end record is outside this file".into());
    }
    let disk = u32_le(bytes, z + 16).ok_or("zip64 end record is outside this file")?;
    let count = u64_le(bytes, z + 32).ok_or("zip64 end record is outside this file")?;
    let size = u64_le(bytes, z + 40).ok_or("zip64 end record is outside this file")?;
    let offset = u64_le(bytes, z + 48).ok_or("zip64 end record is outside this file")?;
    Ok(Dir {
        offset,
        size,
        count,
        zip64: true,
        spanned: spanned || disk != 0,
    })
}

fn read_members(bytes: &[u8], dir: &Dir) -> Result<Vec<Member>, String> {
    let limit = dir.count.min(MEMBER_CAP as u64);
    if limit == 0 {
        return Ok(Vec::new());
    }
    let start = fit(
        bytes,
        dir.offset,
        dir.size,
        "zip central directory is outside this file",
    )?;
    let end = start
        + usize::try_from(dir.size).map_err(|_| "zip central directory is outside this file")?;
    let mut at = start;
    let mut members = Vec::new();
    while (members.len() as u64) < limit {
        let Some(header_end) = at.checked_add(46) else {
            return Err("zip central directory ends early".into());
        };
        if header_end > end {
            return Err("zip central directory ends early".into());
        }
        if bytes.get(at..at + 4) != Some(&CD_SIG) {
            return Err("zip central directory signature is wrong".into());
        }
        let flags = need_u16(bytes, at + 8)?;
        let method = need_u16(bytes, at + 10)?;
        let crc = need_u32(bytes, at + 16)?;
        let compressed = u64::from(need_u32(bytes, at + 20)?);
        let size = u64::from(need_u32(bytes, at + 24)?);
        let name_len = need_u16(bytes, at + 28)? as usize;
        let extra_len = need_u16(bytes, at + 30)? as usize;
        let comment_len = need_u16(bytes, at + 32)? as usize;
        let offset = u64::from(need_u32(bytes, at + 42)?);
        let name_at = at + 46;
        let extra_at = name_at
            .checked_add(name_len)
            .ok_or("zip member is outside this file")?;
        let comment_at = extra_at
            .checked_add(extra_len)
            .ok_or("zip member is outside this file")?;
        let next = comment_at
            .checked_add(comment_len)
            .ok_or("zip member is outside this file")?;
        if next > end {
            return Err("zip member is outside this file".into());
        }
        let extra = &bytes[extra_at..comment_at];
        let (size, compressed, offset) = zip64_sizes(extra, size, compressed, offset);
        let keep = name_len.min(NAME_CAP);
        let name = String::from_utf8_lossy(&bytes[name_at..name_at + keep]).into_owned();
        members.push(Member {
            name,
            method,
            size,
            compressed,
            crc,
            offset,
            encrypted: flags & 1 != 0,
        });
        at = next;
    }
    Ok(members)
}

fn zip64_sizes(extra: &[u8], size: u64, compressed: u64, offset: u64) -> (u64, u64, u64) {
    let mut size = size;
    let mut compressed = compressed;
    let mut offset = offset;
    let mut at = 0;
    while at + 4 <= extra.len() {
        let Some(id) = u16_le(extra, at) else { break };
        let Some(len) = u16_le(extra, at + 2) else {
            break;
        };
        let data_at = at + 4;
        let Some(data_end) = data_at.checked_add(len as usize) else {
            break;
        };
        if data_end > extra.len() {
            break;
        }
        if id == 1 {
            let mut data = &extra[data_at..data_end];
            if size == u64::from(u32::MAX) {
                if let Some(value) = take_u64(&mut data) {
                    size = value;
                }
            }
            if compressed == u64::from(u32::MAX) {
                if let Some(value) = take_u64(&mut data) {
                    compressed = value;
                }
            }
            if offset == u64::from(u32::MAX) {
                if let Some(value) = take_u64(&mut data) {
                    offset = value;
                }
            }
        }
        at = data_end;
    }
    (size, compressed, offset)
}

fn take_u64(data: &mut &[u8]) -> Option<u64> {
    if data.len() < 8 {
        return None;
    }
    let value = u64_le(data, 0)?;
    *data = &data[8..];
    Some(value)
}

fn profile_of(members: &[Member]) -> &'static str {
    if has(members, |name| {
        name == "AndroidManifest.xml" || name.ends_with("/AndroidManifest.xml")
    }) {
        return "apk";
    }
    if has(members, |name| {
        name.starts_with("Payload/") && name.contains(".app/")
    }) {
        return "ipa";
    }
    if has(members, |name| name == "[Content_Types].xml") {
        return "ooxml";
    }
    if has(members, |name| name == "META-INF/container.xml") {
        return "epub";
    }
    if has(members, |name| name == "META-INF/MANIFEST.MF") {
        return "jar";
    }
    if has(members, |name| {
        name.ends_with(".dist-info/METADATA") || name.ends_with(".dist-info/WHEEL")
    }) {
        return "wheel";
    }
    "zip"
}

fn has(members: &[Member], pred: impl Fn(&str) -> bool) -> bool {
    members.iter().any(|row| pred(&row.name))
}

fn has_cd_sig(bytes: &[u8], offset: u64) -> bool {
    let Ok(at) = usize::try_from(offset) else {
        return false;
    };
    bytes.get(at..at + 4) == Some(&CD_SIG)
}

fn fit(bytes: &[u8], offset: u64, len: u64, err: &str) -> Result<usize, String> {
    let start = usize::try_from(offset).map_err(|_| err.to_string())?;
    let len = usize::try_from(len).map_err(|_| err.to_string())?;
    let end = start.checked_add(len).ok_or_else(|| err.to_string())?;
    if end > bytes.len() {
        return Err(err.into());
    }
    Ok(start)
}

fn need_u16(bytes: &[u8], at: usize) -> Result<u16, String> {
    u16_le(bytes, at).ok_or_else(|| "zip member is outside this file".into())
}

fn need_u32(bytes: &[u8], at: usize) -> Result<u32, String> {
    u32_le(bytes, at).ok_or_else(|| "zip member is outside this file".into())
}

fn u16_le(bytes: &[u8], at: usize) -> Option<u16> {
    let s = bytes.get(at..at + 2)?;
    Some(u16::from_le_bytes([s[0], s[1]]))
}

fn u32_le(bytes: &[u8], at: usize) -> Option<u32> {
    let s = bytes.get(at..at + 4)?;
    Some(u32::from_le_bytes([s[0], s[1], s[2], s[3]]))
}

fn u64_le(bytes: &[u8], at: usize) -> Option<u64> {
    let s = bytes.get(at..at + 8)?;
    Some(u64::from_le_bytes([
        s[0], s[1], s[2], s[3], s[4], s[5], s[6], s[7],
    ]))
}

#[cfg(test)]
mod tests {
    use super::*;

    fn store_zip(files: &[(&str, &[u8])]) -> Vec<u8> {
        store_zip_comment(files, b"")
    }

    fn store_zip_comment(files: &[(&str, &[u8])], comment: &[u8]) -> Vec<u8> {
        let mut local = Vec::new();
        let mut central = Vec::new();
        for (name, data) in files {
            let offset = u32::try_from(local.len()).unwrap();
            push_local(&mut local, name, data);
            push_central(&mut central, name, data, offset, 0);
        }
        let cd_offset = u32::try_from(local.len()).unwrap();
        let cd_size = u32::try_from(central.len()).unwrap();
        local.extend(central);
        push_eocd(&mut local, files.len() as u16, cd_size, cd_offset, comment);
        local
    }

    fn push_local(out: &mut Vec<u8>, name: &str, data: &[u8]) {
        out.extend_from_slice(&0x0403_4b50u32.to_le_bytes());
        out.extend_from_slice(&20u16.to_le_bytes());
        out.extend_from_slice(&0u16.to_le_bytes());
        out.extend_from_slice(&0u16.to_le_bytes());
        out.extend_from_slice(&0u16.to_le_bytes());
        out.extend_from_slice(&0u16.to_le_bytes());
        out.extend_from_slice(&0u32.to_le_bytes());
        out.extend_from_slice(&(data.len() as u32).to_le_bytes());
        out.extend_from_slice(&(data.len() as u32).to_le_bytes());
        out.extend_from_slice(&(name.len() as u16).to_le_bytes());
        out.extend_from_slice(&0u16.to_le_bytes());
        out.extend_from_slice(name.as_bytes());
        out.extend_from_slice(data);
    }

    fn push_central(out: &mut Vec<u8>, name: &str, data: &[u8], offset: u32, flags: u16) {
        out.extend_from_slice(&0x0201_4b50u32.to_le_bytes());
        out.extend_from_slice(&20u16.to_le_bytes());
        out.extend_from_slice(&20u16.to_le_bytes());
        out.extend_from_slice(&flags.to_le_bytes());
        out.extend_from_slice(&0u16.to_le_bytes());
        out.extend_from_slice(&0u16.to_le_bytes());
        out.extend_from_slice(&0u16.to_le_bytes());
        out.extend_from_slice(&0u32.to_le_bytes());
        out.extend_from_slice(&(data.len() as u32).to_le_bytes());
        out.extend_from_slice(&(data.len() as u32).to_le_bytes());
        out.extend_from_slice(&(name.len() as u16).to_le_bytes());
        out.extend_from_slice(&0u16.to_le_bytes());
        out.extend_from_slice(&0u16.to_le_bytes());
        out.extend_from_slice(&0u16.to_le_bytes());
        out.extend_from_slice(&0u16.to_le_bytes());
        out.extend_from_slice(&0u32.to_le_bytes());
        out.extend_from_slice(&offset.to_le_bytes());
        out.extend_from_slice(name.as_bytes());
        let _ = data;
    }

    fn push_eocd(out: &mut Vec<u8>, count: u16, cd_size: u32, cd_offset: u32, comment: &[u8]) {
        out.extend_from_slice(&0x0605_4b50u32.to_le_bytes());
        out.extend_from_slice(&0u16.to_le_bytes());
        out.extend_from_slice(&0u16.to_le_bytes());
        out.extend_from_slice(&count.to_le_bytes());
        out.extend_from_slice(&count.to_le_bytes());
        out.extend_from_slice(&cd_size.to_le_bytes());
        out.extend_from_slice(&cd_offset.to_le_bytes());
        out.extend_from_slice(&(comment.len() as u16).to_le_bytes());
        out.extend_from_slice(comment);
    }

    #[test]
    fn a_store_zip_lists_each_member() {
        let bytes = store_zip(&[
            ("META-INF/MANIFEST.MF", b"Manifest-Version: 1.0\n"),
            ("hello.txt", b"hi"),
        ]);
        assert!(is_zip(&bytes));
        let archive = parse(&bytes).unwrap();
        assert_eq!(archive.profile, "jar");
        assert_eq!(archive.count, 2);
        assert_eq!(archive.listed, 2);
        assert!(!archive.zip64);
        assert!(!archive.encrypted);
        assert_eq!(archive.members[0].name, "META-INF/MANIFEST.MF");
        assert_eq!(archive.members[0].method, 0);
        assert_eq!(archive.members[0].size, 22);
        assert_eq!(archive.members[0].compressed, 22);
        assert_eq!(archive.members[0].offset, 0);
        assert_eq!(archive.members[1].name, "hello.txt");
        assert!(archive.members[1].offset > 0);
        let page = page::render("a.jar", &archive, false);
        assert!(page.contains("format: zip"), "{page}");
        assert!(page.contains("profile: jar"), "{page}");
        assert!(page.contains("hello.txt"), "{page}");
        assert!(page.contains("method: store (0)"), "{page}");
        assert!(!page.contains("\nload:"), "{page}");
        let json = page::render_json("a.jar", &archive);
        assert!(json.contains(r#""format":"zip""#), "{json}");
        assert!(json.contains(r#""profile":"jar""#), "{json}");
        assert!(!json.contains(r#""load""#), "{json}");
    }

    #[test]
    fn a_comment_still_finds_the_directory() {
        let bytes = store_zip_comment(&[("a.txt", b"z")], b"hi");
        let archive = parse(&bytes).unwrap();
        assert_eq!(archive.comment, 2);
        assert_eq!(archive.members[0].name, "a.txt");
    }

    #[test]
    fn profiles_come_from_member_names() {
        let cases = [
            ("AndroidManifest.xml", "apk"),
            ("Payload/Demo.app/Demo", "ipa"),
            ("[Content_Types].xml", "ooxml"),
            ("META-INF/container.xml", "epub"),
            ("pkg.dist-info/METADATA", "wheel"),
            ("readme.txt", "zip"),
        ];
        for (name, profile) in cases {
            let archive = parse(&store_zip(&[(name, b"x")])).unwrap();
            assert_eq!(archive.profile, profile, "{name}");
        }
    }

    #[test]
    fn an_encrypted_flag_marks_the_member() {
        let mut local = Vec::new();
        let mut central = Vec::new();
        push_local(&mut local, "secret", b"aa");
        push_central(&mut central, "secret", b"aa", 0, 1);
        let cd_offset = u32::try_from(local.len()).unwrap();
        let cd_size = u32::try_from(central.len()).unwrap();
        local.extend(central);
        push_eocd(&mut local, 1, cd_size, cd_offset, b"");
        let archive = parse(&local).unwrap();
        assert!(archive.encrypted);
        assert!(archive.members[0].encrypted);
        let page = page::render("a.zip", &archive, false);
        assert!(page.contains("encrypted: true"), "{page}");
        assert!(page.contains("Encrypted member."), "{page}");
    }

    #[test]
    fn zip64_sizes_come_from_the_extra_field() {
        let name = b"big.bin";
        let mut central = Vec::new();
        central.extend_from_slice(&0x0201_4b50u32.to_le_bytes());
        central.extend_from_slice(&45u16.to_le_bytes());
        central.extend_from_slice(&45u16.to_le_bytes());
        central.extend_from_slice(&0u16.to_le_bytes());
        central.extend_from_slice(&8u16.to_le_bytes());
        central.extend_from_slice(&0u16.to_le_bytes());
        central.extend_from_slice(&0u16.to_le_bytes());
        central.extend_from_slice(&0u32.to_le_bytes());
        central.extend_from_slice(&u32::MAX.to_le_bytes());
        central.extend_from_slice(&u32::MAX.to_le_bytes());
        central.extend_from_slice(&(name.len() as u16).to_le_bytes());
        central.extend_from_slice(&28u16.to_le_bytes());
        central.extend_from_slice(&0u16.to_le_bytes());
        central.extend_from_slice(&0u16.to_le_bytes());
        central.extend_from_slice(&0u16.to_le_bytes());
        central.extend_from_slice(&0u32.to_le_bytes());
        central.extend_from_slice(&u32::MAX.to_le_bytes());
        central.extend_from_slice(name);
        central.extend_from_slice(&1u16.to_le_bytes());
        central.extend_from_slice(&24u16.to_le_bytes());
        central.extend_from_slice(&100u64.to_le_bytes());
        central.extend_from_slice(&80u64.to_le_bytes());
        central.extend_from_slice(&0u64.to_le_bytes());
        let cd_size = central.len() as u64;
        let mut bytes = central;
        let zoff = bytes.len() as u64;
        bytes.extend_from_slice(&0x0606_4b50u32.to_le_bytes());
        bytes.extend_from_slice(&44u64.to_le_bytes());
        bytes.extend_from_slice(&45u16.to_le_bytes());
        bytes.extend_from_slice(&45u16.to_le_bytes());
        bytes.extend_from_slice(&0u32.to_le_bytes());
        bytes.extend_from_slice(&0u32.to_le_bytes());
        bytes.extend_from_slice(&1u64.to_le_bytes());
        bytes.extend_from_slice(&1u64.to_le_bytes());
        bytes.extend_from_slice(&cd_size.to_le_bytes());
        bytes.extend_from_slice(&0u64.to_le_bytes());
        bytes.extend_from_slice(&0x0706_4b50u32.to_le_bytes());
        bytes.extend_from_slice(&0u32.to_le_bytes());
        bytes.extend_from_slice(&zoff.to_le_bytes());
        bytes.extend_from_slice(&1u32.to_le_bytes());
        bytes.extend_from_slice(&0x0605_4b50u32.to_le_bytes());
        bytes.extend_from_slice(&0u16.to_le_bytes());
        bytes.extend_from_slice(&0u16.to_le_bytes());
        bytes.extend_from_slice(&0xffffu16.to_le_bytes());
        bytes.extend_from_slice(&0xffffu16.to_le_bytes());
        bytes.extend_from_slice(&u32::MAX.to_le_bytes());
        bytes.extend_from_slice(&u32::MAX.to_le_bytes());
        bytes.extend_from_slice(&0u16.to_le_bytes());
        let archive = parse(&bytes).unwrap();
        assert!(archive.zip64);
        assert_eq!(archive.members[0].size, 100);
        assert_eq!(archive.members[0].compressed, 80);
        assert_eq!(archive.members[0].offset, 0);
        assert_eq!(archive.members[0].method, 8);
        let page = page::render("a.zip", &archive, false);
        assert!(page.contains("method: deflate (8)"), "{page}");
        assert!(page.contains("zip64: true"), "{page}");
    }

    #[test]
    fn an_empty_zip_has_no_members() {
        let mut bytes = Vec::new();
        push_eocd(&mut bytes, 0, 0, 0, b"");
        let archive = parse(&bytes).unwrap();
        assert_eq!(archive.count, 0);
        assert!(archive.members.is_empty());
        assert_eq!(archive.profile, "zip");
    }

    #[test]
    fn a_long_directory_stops_at_the_cap() {
        let mut central = Vec::new();
        let count = MEMBER_CAP as u16 + 1;
        for _ in 0..count {
            push_central(&mut central, "a", b"", 0, 0);
        }
        let cd_size = u32::try_from(central.len()).unwrap();
        let mut bytes = central;
        push_eocd(&mut bytes, count, cd_size, 0, b"");
        let archive = parse(&bytes).unwrap();
        assert_eq!(archive.count, u64::from(count));
        assert_eq!(archive.listed, MEMBER_CAP);
        assert_eq!(archive.members.len(), MEMBER_CAP);
    }

    #[test]
    fn hostile_zip_bytes_return_an_error() {
        assert_eq!(parse(b"").unwrap_err(), "not a zip file");
        assert_eq!(parse(b"hello").unwrap_err(), "not a zip file");
        assert!(parse(b"PK\x03\x04").is_err());
        assert!(parse(&[0x50, 0x4b, 0x05, 0x06]).is_err());
        let mut broken = Vec::new();
        push_eocd(&mut broken, 1, 4, 0, b"");
        assert!(parse(&broken).is_err());
        let mut huge = vec![0u8; 64 * 1024];
        huge[..4].copy_from_slice(b"PK\x03\x04");
        assert!(parse(&huge).is_err());
    }
}
