//! Download one https archive. The body is written to a file.

use std::io::{Read, Write};
use std::net::{TcpStream, ToSocketAddrs};
use std::path::Path;
use std::sync::Arc;
use std::time::Duration;

use rustls::pki_types::ServerName;

use hoststub::{https_url, parse_reply, read_chunked};

/// download stores the pin bytes in dest. `on_byte` sees the count so far.
/// A network error or HTTP 500, 502, 503, or 504 tries again. A bad pin does not.
pub fn download(
    url: &str,
    dest: &Path,
    cap: u64,
    on_byte: &mut dyn FnMut(u64),
) -> Result<(), String> {
    let mut last = String::new();
    for attempt in 1..=8 {
        match download_try(url, dest, cap, on_byte) {
            Ok(()) => return Ok(()),
            Err(err) if attempt < 8 && retryable(&err) => {
                last = err;
                let _ = std::fs::remove_file(dest);
                std::thread::sleep(Duration::from_secs(attempt));
            }
            Err(err) => return Err(err),
        }
    }
    Err(last)
}

fn download_try(
    url: &str,
    dest: &Path,
    cap: u64,
    on_byte: &mut dyn FnMut(u64),
) -> Result<(), String> {
    let mut url = url.to_string();
    for _ in 0..5 {
        match fetch_once(&url, dest, cap, on_byte)? {
            Hop::Done => return Ok(()),
            Hop::Redirect(next) => url = next,
        }
    }
    Err("hoststub: too many redirects".to_string())
}

fn retryable(err: &str) -> bool {
    const CODES: &[&str] = &["HTTP 500", "HTTP 502", "HTTP 503", "HTTP 504"];
    if CODES.iter().any(|code| err.contains(code)) {
        return true;
    }
    const STOP: &[&str] = &[
        "HTTP ",
        "larger than the pin",
        "too many redirects",
        "redirect has no url",
        "not a dns name",
        "sha256",
    ];
    !STOP.iter().any(|part| err.contains(part))
}

enum Hop {
    Done,
    Redirect(String),
}

fn fetch_once(
    url: &str,
    dest: &Path,
    cap: u64,
    on_byte: &mut dyn FnMut(u64),
) -> Result<Hop, String> {
    let (host, path) = https_url(url)?;
    let mut tcp = connect(&host)?;
    let name = ServerName::try_from(host.clone())
        .map_err(|_| format!("hoststub: the host is not a dns name: {host}"))?;
    let config = tls_config()?;
    let mut conn =
        rustls::ClientConnection::new(Arc::new(config), name).map_err(|err| err.to_string())?;
    let mut tls = rustls::Stream::new(&mut conn, &mut tcp);
    let req = format!("GET {path} HTTP/1.1\r\nHost: {host}\r\nUser-Agent: devlayer-hoststub\r\nConnection: close\r\n\r\n");
    tls.write_all(req.as_bytes())
        .map_err(|err| err.to_string())?;
    let (header, mut rest) = read_header(&mut tls)?;
    let reply = parse_reply(&header)?;
    if matches!(reply.status, 301 | 302 | 303 | 307 | 308) {
        let location = reply
            .location
            .ok_or_else(|| "hoststub: the redirect has no url".to_string())?;
        return Ok(Hop::Redirect(next_url(&host, &location)?));
    }
    if reply.status != 200 {
        return Err(format!("hoststub: HTTP {}", reply.status));
    }
    if let Some(len) = reply.length {
        if len > cap {
            return Err("hoststub: the archive is larger than the pin".to_string());
        }
    }
    let mut file = std::fs::File::create(dest).map_err(|err| err.to_string())?;
    let mut got = 0u64;
    if !rest.is_empty() {
        let take = bound(&rest, cap, got)?;
        file.write_all(&rest[..take])
            .map_err(|err| err.to_string())?;
        got += take as u64;
        on_byte(got);
        rest.drain(..take);
    }
    if reply.chunked {
        let prefix = std::io::Cursor::new(rest);
        let mut chain = prefix.chain(tls);
        let left = cap - got;
        let n = read_chunked(&mut chain, left, Progress::new(&mut file, on_byte, got))?;
        got += n;
        on_byte(got);
        return Ok(Hop::Done);
    }
    let want = reply.length.unwrap_or(cap);
    while got < want {
        let mut buf = [0u8; 64 * 1024];
        let n = tls.read(&mut buf).map_err(|err| err.to_string())?;
        if n == 0 {
            break;
        }
        let room = (want - got) as usize;
        let take = n.min(room);
        file.write_all(&buf[..take])
            .map_err(|err| err.to_string())?;
        got += take as u64;
        on_byte(got);
        if take < n {
            return Err("hoststub: the archive is larger than the pin".to_string());
        }
    }
    if let Some(len) = reply.length {
        if got != len {
            return Err("hoststub: the archive ended early".to_string());
        }
    }
    Ok(Hop::Done)
}

fn connect(host: &str) -> Result<TcpStream, String> {
    let addr = (host, 443)
        .to_socket_addrs()
        .map_err(|err| err.to_string())?
        .next()
        .ok_or_else(|| format!("hoststub: {host} has no address"))?;
    let tcp = TcpStream::connect_timeout(&addr, Duration::from_secs(20))
        .map_err(|err| err.to_string())?;
    tcp.set_read_timeout(Some(Duration::from_secs(60)))
        .map_err(|err| err.to_string())?;
    tcp.set_write_timeout(Some(Duration::from_secs(60)))
        .map_err(|err| err.to_string())?;
    Ok(tcp)
}

fn tls_config() -> Result<rustls::ClientConfig, String> {
    let roots = rustls::RootCertStore::from_iter(webpki_roots::TLS_SERVER_ROOTS.iter().cloned());
    Ok(rustls::ClientConfig::builder()
        .with_root_certificates(roots)
        .with_no_client_auth())
}

fn read_header(input: &mut impl Read) -> Result<(String, Vec<u8>), String> {
    let mut buf = Vec::new();
    let mut byte = [0u8; 1];
    loop {
        let n = input.read(&mut byte).map_err(|err| err.to_string())?;
        if n == 0 {
            return Err("hoststub: the reply ended early".to_string());
        }
        buf.push(byte[0]);
        if buf.len() > 64 * 1024 {
            return Err("hoststub: the reply header is too long".to_string());
        }
        if buf.ends_with(b"\r\n\r\n") {
            let header = String::from_utf8(buf[..buf.len() - 4].to_vec())
                .map_err(|_| "hoststub: the reply header is broken".to_string())?;
            return Ok((header, Vec::new()));
        }
    }
}

fn next_url(host: &str, location: &str) -> Result<String, String> {
    if let Some(rest) = location.strip_prefix("https://") {
        if rest.is_empty() {
            return Err("hoststub: the redirect has no url".to_string());
        }
        return Ok(location.to_string());
    }
    if location.starts_with('/') && !location.starts_with("//") {
        return Ok(format!("https://{host}{location}"));
    }
    Err("hoststub: the redirect has no url".to_string())
}

#[cfg(test)]
mod tests {
    use super::retryable;

    #[test]
    fn retry_keeps_a_blip_and_stops_a_bad_pin() {
        assert!(retryable("hoststub: HTTP 500"));
        assert!(retryable("hoststub: HTTP 502"));
        assert!(retryable("hoststub: HTTP 503"));
        assert!(retryable("hoststub: HTTP 504"));
        assert!(retryable("connection reset"));
        assert!(retryable("hoststub: the archive ended early"));
        assert!(!retryable("hoststub: HTTP 404"));
        assert!(!retryable("hoststub: the archive is larger than the pin"));
        assert!(!retryable("hoststub: too many redirects"));
        assert!(!retryable("hoststub: the redirect has no url"));
        assert!(!retryable("hoststub: the host is not a dns name: x"));
        assert!(!retryable("hoststub: sha256 abc"));
    }
}

fn bound(buf: &[u8], cap: u64, got: u64) -> Result<usize, String> {
    let room = cap.saturating_sub(got) as usize;
    if buf.len() > room {
        return Err("hoststub: the archive is larger than the pin".to_string());
    }
    Ok(buf.len())
}

struct Progress<'a, W> {
    out: &'a mut W,
    on_byte: &'a mut dyn FnMut(u64),
    got: u64,
}

impl<'a, W: Write> Progress<'a, W> {
    fn new(out: &'a mut W, on_byte: &'a mut dyn FnMut(u64), got: u64) -> Self {
        Self { out, on_byte, got }
    }
}

impl<W: Write> Write for Progress<'_, W> {
    fn write(&mut self, buf: &[u8]) -> std::io::Result<usize> {
        let n = self.out.write(buf)?;
        self.got += n as u64;
        (self.on_byte)(self.got);
        Ok(n)
    }
    fn flush(&mut self) -> std::io::Result<()> {
        self.out.flush()
    }
}
