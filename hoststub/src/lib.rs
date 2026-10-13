//! The stub downloads one pinned archive when the stamp is absent.
//! The archive stays on the host.

mod catalog;

pub use catalog::{find, machine, pin, App, HostLib, Pack, Pin, APPS};

use sha2::Digest;
use std::fs::{self, File};
use std::io::{self, Read, Write};
use std::path::{Component, Path, PathBuf};

pub const STAMP: &str = ".devlayer-sha256";

/// Action is the next step for one run.
pub enum Action {
    Run(PathBuf),
    Fetch { cache: PathBuf },
}

/// decide runs a stamped tree or asks for a download.
pub fn decide(data: &Path, app: &App, sum: &str) -> Action {
    let cache = data
        .join("devlayer")
        .join("host")
        .join(app.name)
        .join(app.version);
    if stamp_matches(&cache, sum) {
        if let Some(path) = ready_exec(&cache, app.exec) {
            return Action::Run(path);
        }
    }
    Action::Fetch { cache }
}

/// write_stamp records the pin after the exec file is in the tree.
pub fn write_stamp(cache: &Path, sum: &str, exec_rel: &str) -> Result<(), String> {
    if ready_exec(cache, exec_rel).is_none() {
        return Err(format!("hoststub: {exec_rel} is missing"));
    }
    if !sha_ok(sum) {
        return Err("hoststub: sha256 must be 64 hex chars".to_string());
    }
    let path = cache.join(STAMP);
    fs::write(&path, format!("{sum}\n")).map_err(|err| err.to_string())?;
    Ok(())
}

/// entry_rel drops `strip` leading parts and refuses a path that leaves the tree.
pub fn entry_rel(name: &str, strip: usize) -> Result<Option<String>, String> {
    if name.contains('\0') || name.contains('\n') || name.contains('\r') {
        return Err(format!("hoststub: archive path leaves the tree: {name}"));
    }
    let name = name.replace('\\', "/");
    if name.starts_with('/') {
        return Err(format!("hoststub: archive path leaves the tree: {name}"));
    }
    let parts: Vec<&str> = name
        .split('/')
        .filter(|part| !part.is_empty() && *part != ".")
        .collect();
    if parts.iter().any(|part| *part == "..") {
        return Err(format!("hoststub: archive path leaves the tree: {name}"));
    }
    if parts.len() <= strip {
        return Ok(None);
    }
    Ok(Some(parts[strip..].join("/")))
}

/// extract unpacks a .tar.gz into dest. `strip` drops leading parts.
pub fn extract(tar_gz: &Path, dest: &Path, strip: usize, exec_rel: &str) -> Result<(), String> {
    let file = File::open(tar_gz).map_err(|err| err.to_string())?;
    let dec = flate2::read::GzDecoder::new(file);
    let mut archive = tar::Archive::new(dec);
    let entries = archive.entries().map_err(|err| err.to_string())?;
    fs::create_dir_all(dest).map_err(|err| err.to_string())?;
    for entry in entries {
        let mut entry = entry.map_err(|err| err.to_string())?;
        let raw = entry
            .path()
            .map_err(|err| err.to_string())?
            .to_string_lossy()
            .into_owned();
        let Some(rel) = entry_rel(&raw, strip)? else {
            continue;
        };
        let place = dest.join(&rel);
        let kind = entry.header().entry_type();
        if kind.is_dir() {
            fs::create_dir_all(&place).map_err(|err| err.to_string())?;
            continue;
        }
        if kind.is_symlink() {
            let link = entry
                .link_name()
                .map_err(|err| err.to_string())?
                .ok_or_else(|| format!("hoststub: symlink {rel} has no target"))?;
            let link = link.to_string_lossy();
            if !link_stays(&rel, &link) {
                return Err(format!("hoststub: archive path leaves the tree: {link}"));
            }
            if let Some(parent) = place.parent() {
                fs::create_dir_all(parent).map_err(|err| err.to_string())?;
            }
            let _ = fs::remove_file(&place);
            std::os::unix::fs::symlink(link.as_ref(), &place).map_err(|err| err.to_string())?;
            continue;
        }
        if !kind.is_file() {
            return Err(format!("hoststub: archive entry {rel} is not a file"));
        }
        if let Some(parent) = place.parent() {
            fs::create_dir_all(parent).map_err(|err| err.to_string())?;
        }
        let mut out = File::create(&place).map_err(|err| err.to_string())?;
        io::copy(&mut entry, &mut out).map_err(|err| err.to_string())?;
        let mode = entry.header().mode().unwrap_or(0o644);
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            fs::set_permissions(&place, fs::Permissions::from_mode(mode))
                .map_err(|err| err.to_string())?;
        }
    }
    if ready_exec(dest, exec_rel).is_none() {
        return Err(format!("hoststub: {exec_rel} is missing"));
    }
    Ok(())
}

/// inflate writes one gzip body to dest and stops at `plain` bytes.
pub fn inflate(src: &Path, dest: &Path, plain: u64) -> Result<(), String> {
    if plain == 0 {
        return Err("hoststub: the file has no size".to_string());
    }
    let file = File::open(src).map_err(|err| err.to_string())?;
    let mut dec = flate2::read::GzDecoder::new(file);
    if let Some(parent) = dest.parent() {
        fs::create_dir_all(parent).map_err(|err| err.to_string())?;
    }
    let mut out = File::create(dest).map_err(|err| err.to_string())?;
    let mut got = 0u64;
    let mut buf = [0u8; 64 * 1024];
    loop {
        let n = dec.read(&mut buf).map_err(|err| err.to_string())?;
        if n == 0 {
            break;
        }
        if got.saturating_add(n as u64) > plain {
            let _ = fs::remove_file(dest);
            return Err("hoststub: the file is larger than the pin".to_string());
        }
        out.write_all(&buf[..n]).map_err(|err| err.to_string())?;
        got += n as u64;
    }
    if got != plain {
        let _ = fs::remove_file(dest);
        return Err("hoststub: the file ended early".to_string());
    }
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        fs::set_permissions(dest, fs::Permissions::from_mode(0o755))
            .map_err(|err| err.to_string())?;
    }
    Ok(())
}

/// installed_exec is the program Termium setup leaves in its home.
pub fn installed_exec(home: &Path, exec_rel: &str) -> PathBuf {
    home.join("current").join(exec_rel)
}

/// park_launcher moves a regular file aside for `body`, then puts that file back.
/// A symlink that `body` writes at the same path is removed first.
pub fn park_launcher(
    launcher: &Path,
    aside: &Path,
    body: impl FnOnce() -> Result<(), String>,
) -> Result<(), String> {
    let occupied = fs::symlink_metadata(launcher)
        .map(|meta| meta.is_file())
        .unwrap_or(false);
    if occupied {
        if let Some(parent) = aside.parent() {
            fs::create_dir_all(parent).map_err(|err| err.to_string())?;
        }
        fs::rename(launcher, aside).map_err(|err| err.to_string())?;
    }
    let result = body();
    if occupied {
        if fs::symlink_metadata(launcher)
            .map(|meta| meta.file_type().is_symlink())
            .unwrap_or(false)
        {
            let _ = fs::remove_file(launcher);
        }
        if let Err(err) = fs::rename(aside, launcher) {
            return Err(format!("hoststub: the command did not return: {err}"));
        }
    }
    result
}

/// progress_line is one status line. The numbers are decimal megabytes.
pub fn progress_line(name: &str, version: &str, got: u64, total: u64) -> String {
    format!("{name} {version}  {} / {} MB", mb(got), mb(total))
}

/// sha256_file hashes one file.
pub fn sha256_file(path: &Path) -> Result<String, String> {
    let mut file = File::open(path).map_err(|err| err.to_string())?;
    let mut hasher = sha2::Sha256::new();
    let mut buf = [0u8; 64 * 1024];
    loop {
        let n = file.read(&mut buf).map_err(|err| err.to_string())?;
        if n == 0 {
            break;
        }
        hasher.update(&buf[..n]);
    }
    Ok(hex(&hasher.finalize()))
}

/// parse_reply reads one HTTP status block.
pub fn parse_reply(header: &str) -> Result<Reply, String> {
    let mut lines = header.split("\r\n");
    let status_line = lines.next().unwrap_or("");
    let mut parts = status_line.split(' ');
    let http = parts.next().unwrap_or("");
    if !http.starts_with("HTTP/") {
        return Err("hoststub: the reply has no status".to_string());
    }
    let code = parts
        .next()
        .ok_or_else(|| "hoststub: the reply has no status".to_string())?;
    let status: u16 = code
        .parse()
        .map_err(|_| "hoststub: the reply has no status".to_string())?;
    let mut location = None;
    let mut length = None;
    let mut chunked = false;
    for line in lines {
        if line.is_empty() {
            break;
        }
        let Some((key, value)) = line.split_once(':') else {
            return Err("hoststub: the reply header is broken".to_string());
        };
        let key = key.trim();
        let value = value.trim();
        if key.eq_ignore_ascii_case("location") {
            if value.is_empty() || value.contains(' ') {
                return Err("hoststub: the redirect has no url".to_string());
            }
            location = Some(value.to_string());
        } else if key.eq_ignore_ascii_case("content-length") {
            let n = parse_len(value)?;
            if let Some(prev) = length {
                if prev != n {
                    return Err("hoststub: the reply has two lengths".to_string());
                }
            }
            length = Some(n);
        } else if key.eq_ignore_ascii_case("transfer-encoding")
            && value.eq_ignore_ascii_case("chunked")
        {
            chunked = true;
        }
    }
    Ok(Reply {
        status,
        location,
        length,
        chunked,
    })
}

/// Reply is the part of an HTTP response the stub reads.
pub struct Reply {
    pub status: u16,
    pub location: Option<String>,
    pub length: Option<u64>,
    pub chunked: bool,
}

/// read_chunked copies one chunked body. `cap` is the maximum byte count.
pub fn read_chunked(mut input: impl Read, cap: u64, mut out: impl Write) -> Result<u64, String> {
    let mut got = 0u64;
    loop {
        let line = read_line(&mut input, 64)?;
        let size_txt = line.split(';').next().unwrap_or("").trim();
        if size_txt.is_empty() || !size_txt.bytes().all(|b| b.is_ascii_hexdigit()) {
            return Err("hoststub: the chunk size is broken".to_string());
        }
        let size = u64::from_str_radix(size_txt, 16)
            .map_err(|_| "hoststub: the chunk size is broken".to_string())?;
        if size == 0 {
            return Ok(got);
        }
        if got.saturating_add(size) > cap {
            return Err("hoststub: the archive is larger than the pin".to_string());
        }
        copy_exact(&mut input, &mut out, size)?;
        got += size;
        let crlf = read_line(&mut input, 8)?;
        if crlf != "" && crlf != "\r" {
            return Err("hoststub: the chunk ending is broken".to_string());
        }
    }
}

/// https_url splits one https URL into host and path.
pub fn https_url(url: &str) -> Result<(String, String), String> {
    let rest = url
        .strip_prefix("https://")
        .ok_or_else(|| format!("hoststub: the url is not https: {url}"))?;
    if rest.contains('@') {
        return Err("hoststub: the url has a user".to_string());
    }
    let (hostport, path) = match rest.split_once('/') {
        Some((host, path)) => (host, format!("/{path}")),
        None => (rest, "/".to_string()),
    };
    if hostport.is_empty() || hostport.contains('/') {
        return Err("hoststub: the url has no host".to_string());
    }
    let host = hostport.split(':').next().unwrap_or("").to_string();
    if host.is_empty() || hostport.contains(':') && hostport != format!("{host}:443") {
        return Err("hoststub: the url port is not 443".to_string());
    }
    Ok((host, path))
}

fn stamp_matches(cache: &Path, sum: &str) -> bool {
    let Ok(text) = fs::read(cache.join(STAMP)) else {
        return false;
    };
    if text.len() > 65 {
        return false;
    }
    let text = String::from_utf8_lossy(&text);
    text.trim_end_matches('\n') == sum
}

fn ready_exec(root: &Path, rel: &str) -> Option<PathBuf> {
    let rel = entry_rel(rel, 0).ok()??;
    let place = root.join(&rel);
    let meta = fs::symlink_metadata(&place).ok()?;
    if meta.file_type().is_symlink() {
        let link = fs::read_link(&place).ok()?;
        let link = link.to_string_lossy();
        if !link_stays(&rel, &link) {
            return None;
        }
    }
    let meta = fs::metadata(&place).ok()?;
    if meta.is_file() {
        Some(place)
    } else {
        None
    }
}

fn link_stays(place_rel: &str, link: &str) -> bool {
    if link.contains('\0') || link.starts_with('/') || link.contains('\\') {
        return false;
    }
    let parent = Path::new(place_rel).parent().unwrap_or(Path::new(""));
    let combined = parent.join(link);
    depth_ok(&combined)
}

fn depth_ok(path: &Path) -> bool {
    let mut depth = 0i32;
    for part in path.components() {
        match part {
            Component::Normal(_) => depth += 1,
            Component::ParentDir => {
                depth -= 1;
                if depth < 0 {
                    return false;
                }
            }
            Component::CurDir => {}
            _ => return false,
        }
    }
    true
}

fn sha_ok(sum: &str) -> bool {
    sum.len() == 64 && sum.bytes().all(|b| b.is_ascii_hexdigit())
}

fn mb(n: u64) -> String {
    format!("{:.1}", n as f64 / 1_000_000.0)
}

fn hex(bytes: &[u8]) -> String {
    const HEX: &[u8] = b"0123456789abcdef";
    let mut out = String::with_capacity(bytes.len() * 2);
    for byte in bytes {
        out.push(HEX[(byte >> 4) as usize] as char);
        out.push(HEX[(byte & 0xf) as usize] as char);
    }
    out
}

fn parse_len(value: &str) -> Result<u64, String> {
    if value.is_empty() || !value.bytes().all(|b| b.is_ascii_digit()) {
        return Err("hoststub: the length is broken".to_string());
    }
    value
        .parse()
        .map_err(|_| "hoststub: the length is broken".to_string())
}

fn read_line(input: &mut impl Read, cap: usize) -> Result<String, String> {
    let mut buf = Vec::new();
    let mut byte = [0u8; 1];
    loop {
        let n = input.read(&mut byte).map_err(|err| err.to_string())?;
        if n == 0 {
            return Err("hoststub: the chunk size is broken".to_string());
        }
        if byte[0] == b'\n' {
            break;
        }
        if buf.len() >= cap {
            return Err("hoststub: the chunk size is broken".to_string());
        }
        if byte[0] != b'\r' {
            buf.push(byte[0]);
        }
    }
    String::from_utf8(buf).map_err(|_| "hoststub: the chunk size is broken".to_string())
}

fn copy_exact(input: &mut impl Read, out: &mut impl Write, size: u64) -> Result<(), String> {
    let mut left = size;
    let mut buf = [0u8; 64 * 1024];
    while left > 0 {
        let want = buf.len().min(left as usize);
        let n = input
            .read(&mut buf[..want])
            .map_err(|err| err.to_string())?;
        if n == 0 {
            return Err("hoststub: the archive ended early".to_string());
        }
        out.write_all(&buf[..n]).map_err(|err| err.to_string())?;
        left -= n as u64;
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::Cursor;

    #[test]
    fn pins_match_the_installer() {
        let app = find("terminal-browser").unwrap();
        let want = [
            (
                "darwin",
                "arm64",
                "f017230c78c60a07ef4451a1eb0a92727f0b955a8fcd87aec358910c5d0c03c7",
                141_704_278,
            ),
            (
                "darwin",
                "x64",
                "01bc6991bad122f42e4f2a5164a198d8384b944c036de112078fd51f58dc67ed",
                149_889_617,
            ),
            (
                "linux",
                "arm64",
                "0cf567d8218995a24fb6ce4b06c07ccf58a8c517906058087355f1e2fad1969f",
                138_044_592,
            ),
            (
                "linux",
                "x64",
                "6277daaabab16711ab3f1961cdffad9efac5e70ac55d5076e2c86496d649d3a4",
                136_646_100,
            ),
        ];
        for (os, arch, sum, size) in want {
            let pin = pin(app, os, arch).unwrap();
            assert_eq!(pin.sha256, sum);
            assert_eq!(pin.size, size);
            assert!(pin.url.contains(app.version));
            assert!(pin.url.starts_with("https://"));
        }
        assert!(pin(app, "windows", "x64").is_err());
        assert!(find("missing").is_err());
    }

    #[test]
    fn decide_fetches_when_the_stamp_is_absent() {
        let dir = scratch("absent");
        let app = find("terminal-browser").unwrap();
        match decide(&dir, app, app.pins[0].sha256) {
            Action::Fetch { .. } => {}
            Action::Run(path) => panic!("ran {path:?}"),
        }
        let _ = fs::remove_dir_all(dir);
    }

    #[test]
    fn decide_runs_a_stamped_tree() {
        let dir = scratch("stamped");
        let app = find("terminal-browser").unwrap();
        let cache = dir
            .join("devlayer")
            .join("host")
            .join(app.name)
            .join(app.version);
        write_fake(&cache);
        write_stamp(&cache, app.pins[0].sha256, app.exec).unwrap();
        match decide(&dir, app, app.pins[0].sha256) {
            Action::Run(path) => assert_eq!(path, cache.join(app.exec)),
            Action::Fetch { .. } => panic!("fetched"),
        }
        let _ = fs::remove_dir_all(dir);
    }

    #[test]
    fn decide_ignores_a_bad_stamp_and_an_outside_symlink() {
        let dir = scratch("bad");
        let app = find("terminal-browser").unwrap();
        let cache = dir
            .join("devlayer")
            .join("host")
            .join(app.name)
            .join(app.version);
        write_fake(&cache);
        fs::write(cache.join(STAMP), "00\n").unwrap();
        match decide(&dir, app, app.pins[0].sha256) {
            Action::Fetch { .. } => {}
            Action::Run(_) => panic!("ran a bad stamp"),
        }
        let outside = scratch("outside");
        fs::write(outside.join("x"), b"#!/bin/sh\n").unwrap();
        let _ = fs::remove_file(cache.join(app.exec));
        std::os::unix::fs::symlink(outside.join("x"), cache.join(app.exec)).unwrap();
        fs::write(cache.join(STAMP), format!("{}\n", app.pins[0].sha256)).unwrap();
        match decide(&dir, app, app.pins[0].sha256) {
            Action::Fetch { .. } => {}
            Action::Run(_) => panic!("ran an outside symlink"),
        }
        let _ = fs::remove_dir_all(dir);
        let _ = fs::remove_dir_all(outside);
    }

    #[test]
    fn extract_refuses_a_path_that_leaves_the_tree() {
        assert!(entry_rel("../x", 0).is_err());
        assert!(entry_rel("/etc/passwd", 0).is_err());
        assert!(entry_rel("a/../../b", 0).is_err());
        assert!(entry_rel("ok/file\0", 0).is_err());
        assert_eq!(
            entry_rel("terminal-browser/bin/terminal-browser", 1)
                .unwrap()
                .unwrap(),
            "bin/terminal-browser"
        );
        let dir = scratch("tar");
        let bad = dir.join("bad.tar.gz");
        write_link_tar(
            &bad,
            "terminal-browser/bin/terminal-browser",
            "../../../../outside",
        );
        let dest = dir.join("out");
        assert!(extract(&bad, &dest, 1, "bin/terminal-browser").is_err());
        let good = dir.join("good.tar.gz");
        write_tar(
            &good,
            &[("terminal-browser/bin/terminal-browser", b"#!/bin/sh\n")],
        );
        let dest = dir.join("good");
        extract(&good, &dest, 1, "bin/terminal-browser").unwrap();
        assert_eq!(
            fs::read(dest.join("bin/terminal-browser")).unwrap(),
            b"#!/bin/sh\n"
        );
        let _ = fs::remove_dir_all(dir);
    }

    #[test]
    fn reply_and_chunks_refuse_broken_bytes() {
        let reply = parse_reply("HTTP/1.1 200 OK\r\nContent-Length: 4\r\n\r\n").unwrap();
        assert_eq!(reply.status, 200);
        assert_eq!(reply.length, Some(4));
        assert!(parse_reply("nope\r\n\r\n").is_err());
        assert!(
            parse_reply("HTTP/1.1 200 OK\r\nContent-Length: 1\r\nContent-Length: 2\r\n\r\n")
                .is_err()
        );
        assert!(parse_reply("HTTP/1.1 200 OK\r\nContent-Length: -1\r\n\r\n").is_err());
        let mut out = Vec::new();
        read_chunked(Cursor::new(b"4\r\nwxyz\r\n0\r\n\r\n"), 10, &mut out).unwrap();
        assert_eq!(out, b"wxyz");
        let mut out = Vec::new();
        assert!(read_chunked(Cursor::new(b"5\r\nhi"), 10, &mut out).is_err());
        let mut out = Vec::new();
        assert!(read_chunked(Cursor::new(b"100\r\n"), 4, &mut out).is_err());
        assert!(https_url("http://terminal-browser.sh/a").is_err());
        assert!(https_url("https://user@terminal-browser.sh/a").is_err());
        let (host, path) = https_url("https://terminal-browser.sh/install/dl/a.tar.gz").unwrap();
        assert_eq!(host, "terminal-browser.sh");
        assert_eq!(path, "/install/dl/a.tar.gz");
    }

    #[test]
    fn grok_pin_matches_the_machine() {
        let app = find("grok").unwrap();
        assert_eq!(find("agent").unwrap().name, "grok");
        assert!(matches!(app.pack, Pack::Gzip));
        assert!(app.arg0);
        let want = [
            (
                "darwin",
                "arm64",
                "macos-aarch64",
                "6e37ef32bfc99173644284566fd8243f3ee573a3ded373a07f8d3782e6c3a95f",
                69_718_933,
                172_237_072,
            ),
            (
                "darwin",
                "x64",
                "macos-x86_64",
                "886182c4df90b1f51bb5d349dc31496c6265773c8aeac0c36d018e3f500be7a8",
                75_142_002,
                185_857_856,
            ),
            (
                "linux",
                "arm64",
                "linux-aarch64",
                "f56c5210eabb3dba1977715454c6c2466ae74dea9a41d5710a2d333160492ef5",
                69_055_281,
                160_670_952,
            ),
            (
                "linux",
                "x64",
                "linux-x86_64",
                "28234f82ce7f24994d6902d102b8c9b135e2e8c75ee492f2d944a2cf0b081c79",
                73_985_594,
                188_326_752,
            ),
        ];
        let mut seen = Vec::new();
        for (os, arch, token, sum, size, plain) in want {
            let pin = pin(app, os, arch).unwrap();
            assert_eq!(pin.sha256, sum);
            assert_eq!(pin.size, size);
            assert_eq!(pin.plain, plain);
            assert_eq!(pin.url, format!("https://x.ai/cli/grok-1.0.52-{token}.gz"));
            assert!(!seen.contains(&pin.url));
            seen.push(pin.url);
        }
        assert!(pin(app, "windows", "x64").is_err());
        assert_ne!(
            pin(app, "darwin", "arm64").unwrap().url,
            pin(app, "linux", "x64").unwrap().url
        );
    }

    #[test]
    fn termium_pin_matches_the_machine() {
        let app = find("termium").unwrap();
        assert!(app.install);
        assert!(matches!(app.pack, Pack::TarGz { strip: 0 }));
        assert_eq!(app.exec, "bin/termium");
        assert_eq!(app.version, "v0.2.4");
        let want = [
            (
                "darwin",
                "arm64",
                "darwin-arm64",
                "1230a1956163bf66ed29de551434245fd8bc7f24e102c46517be4d5b0364a64f",
                50_684_482,
            ),
            (
                "darwin",
                "x64",
                "darwin-amd64",
                "f73a92f9904570fe1913469e6cc478d85af473b30491481d7ae812d8e306bd1b",
                52_225_792,
            ),
            (
                "linux",
                "x64",
                "linux-amd64",
                "1f4d9ef2b19533d1429be98df6020af2f149ef20e685db77e7402f7544fad175",
                179_532_692,
            ),
        ];
        for (os, arch, token, sum, size) in want {
            let pin = pin(app, os, arch).unwrap();
            assert_eq!(pin.sha256, sum);
            assert_eq!(pin.size, size);
            assert_eq!(pin.plain, 0);
            assert_eq!(
                pin.url,
                format!("https://github.com/codr1/termium/releases/download/v0.2.4/termium-{token}.tar.gz")
            );
        }
        assert!(pin(app, "linux", "arm64").is_err());
        assert!(pin(app, "windows", "x64").is_err());
        assert!(!find("terminal-browser").unwrap().install);
        assert!(!find("grok").unwrap().install);
    }

    #[test]
    fn simserve_is_a_darwin_citizen() {
        let app = find("simserve").unwrap();
        assert_eq!(app.version, "0.1.0");
        assert_eq!(app.exec, "simserve");
        assert!(app.aliases.is_empty());
        assert!(!app.arg0);
        assert!(!app.install);
        assert!(matches!(app.pack, Pack::Gzip));
        let got = pin(app, "darwin", "arm64").unwrap();
        assert_eq!(
            got.url,
            "https://github.com/stefanpenner/lcc.live/releases/download/simserve-0.1.0/simserve-darwin-arm64.gz"
        );
        assert_eq!(
            got.sha256,
            "ef242a999e103551408e8d579424e278964ec96badb71ff46062807ef93f8378"
        );
        assert_eq!(got.size, 298_822);
        assert_eq!(got.plain, 723_488);
        assert!(pin(app, "linux", "arm64").is_err());
        assert!(pin(app, "linux", "x64").is_err());
        assert!(pin(app, "darwin", "x64").is_err());
        assert!(pin(app, "windows", "x64").is_err());
        let names: Vec<_> = app.links.iter().map(|row| row.name).collect();
        assert_eq!(
            names,
            [
                "/usr/lib/libobjc.A.dylib",
                "/System/Library/Frameworks/IOSurface.framework/Versions/A/IOSurface",
                "/System/Library/Frameworks/CoreFoundation.framework/Versions/A/CoreFoundation",
                "/System/Library/Frameworks/CoreGraphics.framework/Versions/A/CoreGraphics",
                "/System/Library/Frameworks/ImageIO.framework/Versions/A/ImageIO",
                "/usr/lib/libSystem.B.dylib",
                "/Library/Developer/PrivateFrameworks/CoreSimulator.framework/CoreSimulator",
                "SimulatorKit.framework/SimulatorKit",
            ]
        );
        assert!(app.links[2].why.contains("later pin"));
        assert!(app.links[3].why.contains("later pin"));
        assert!(app.links[4].why.contains("later pin"));
        assert!(app.links[6].why.contains("by name"));
        assert!(app.links[7].why.contains("by name"));
        assert!(find("terminal-browser").unwrap().links.is_empty());
        assert!(find("grok").unwrap().links.is_empty());
        assert!(find("termium").unwrap().links.is_empty());
    }

    #[test]
    fn park_launcher_puts_the_stub_back() {
        let dir = scratch("park");
        let launcher = dir.join("termium");
        let aside = dir.join("termium.hoststub");
        fs::write(&launcher, b"stub").unwrap();
        park_launcher(&launcher, &aside, || {
            std::os::unix::fs::symlink("real", &launcher).unwrap();
            Err("hoststub: setup failed".to_string())
        })
        .unwrap_err();
        assert_eq!(fs::read(&launcher).unwrap(), b"stub");
        assert!(!aside.exists());
        let home = dir.join("home");
        assert_eq!(
            installed_exec(&home, "bin/termium"),
            home.join("current/bin/termium")
        );
        let _ = fs::remove_dir_all(dir);
    }

    #[test]
    fn inflate_writes_one_program() {
        let dir = scratch("gz");
        let src = dir.join("grok.gz");
        let mut enc = flate2::write::GzEncoder::new(Vec::new(), flate2::Compression::default());
        enc.write_all(b"#!/bin/sh\n").unwrap();
        fs::write(&src, enc.finish().unwrap()).unwrap();
        let dest = dir.join("out").join("grok");
        inflate(&src, &dest, 10).unwrap();
        assert_eq!(fs::read(&dest).unwrap(), b"#!/bin/sh\n");
        let mode = {
            use std::os::unix::fs::PermissionsExt;
            fs::metadata(&dest).unwrap().permissions().mode() & 0o777
        };
        assert_eq!(mode, 0o755);
        assert!(inflate(&src, &dir.join("big"), 4).is_err());
        fs::write(&dir.join("cut.gz"), [0x1f, 0x8b, 0x08]).unwrap();
        assert!(inflate(&dir.join("cut.gz"), &dir.join("cut"), 10).is_err());
        assert!(inflate(&src, &dir.join("zero"), 0).is_err());
        let _ = fs::remove_dir_all(dir);
    }

    #[test]
    fn progress_line_names_the_app() {
        let line = progress_line("terminal-browser", "v0.13.4", 1_000_000, 141_704_278);
        assert_eq!(line, "terminal-browser v0.13.4  1.0 / 141.7 MB");
    }

    #[test]
    fn nasty_bytes_do_not_panic() {
        let samples: &[&[u8]] = &[b"", b"\0", b"../", b"HTTP/1.1", b"\r\n\r\n", &[0xff; 80]];
        for sample in samples {
            let text = String::from_utf8_lossy(sample);
            let _ = parse_reply(&text);
            let _ = entry_rel(&text, 1);
            let _ = https_url(&text);
            let _ = read_chunked(Cursor::new(*sample), 32, Vec::new());
        }
        let mut state = 0x1234_5678_u64;
        for _ in 0..400 {
            state = state.wrapping_mul(6364136223846793005).wrapping_add(1);
            let n = (state % 60) as usize;
            let bytes: Vec<u8> = (0..n)
                .map(|i| ((state >> (i % 8)) as u8).wrapping_add(i as u8))
                .collect();
            let text = String::from_utf8_lossy(&bytes);
            let _ = std::panic::catch_unwind(|| {
                let _ = parse_reply(&text);
                let _ = entry_rel(&text, 1);
                let _ = read_chunked(Cursor::new(bytes.clone()), 64, Vec::new());
            })
            .map_err(|_| ())
            .unwrap();
        }
    }

    fn scratch(name: &str) -> PathBuf {
        let path = std::env::temp_dir().join(format!("hoststub-{name}-{}", std::process::id()));
        let _ = fs::remove_dir_all(&path);
        fs::create_dir_all(&path).unwrap();
        path
    }

    fn write_fake(dir: &Path) {
        fs::create_dir_all(dir.join("bin")).unwrap();
        fs::write(dir.join("bin/terminal-browser"), b"#!/bin/sh\n").unwrap();
    }

    fn write_link_tar(path: &Path, name: &str, link: &str) {
        let file = File::create(path).unwrap();
        let enc = flate2::write::GzEncoder::new(file, flate2::Compression::fast());
        let mut builder = tar::Builder::new(enc);
        let mut header = tar::Header::new_gnu();
        header.set_entry_type(tar::EntryType::Symlink);
        header.set_path(name).unwrap();
        header.set_link_name(link).unwrap();
        header.set_size(0);
        header.set_cksum();
        builder.append(&header, std::io::empty()).unwrap();
        builder.finish().unwrap();
    }

    fn write_tar(path: &Path, files: &[(&str, &[u8])]) {
        let file = File::create(path).unwrap();
        let enc = flate2::write::GzEncoder::new(file, flate2::Compression::fast());
        let mut builder = tar::Builder::new(enc);
        for (name, bytes) in files {
            let mut header = tar::Header::new_gnu();
            header.set_path(name).unwrap();
            header.set_size(bytes.len() as u64);
            header.set_mode(0o644);
            header.set_cksum();
            builder.append(&header, *bytes).unwrap();
        }
        builder.finish().unwrap();
    }
}
