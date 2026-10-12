//! Run one pinned host app. Download the archive when the stamp is absent.

mod get;

use std::env;
use std::fs;
use std::io::{self, IsTerminal, Write};
use std::os::unix::process::CommandExt;
use std::path::{Path, PathBuf};
use std::process::Command;

use hoststub::{
    decide, extract, find, inflate, machine, pin, progress_line, sha256_file, write_stamp, Action,
    App, Pack,
};

fn main() {
    if let Err(err) = run() {
        eprintln!("error: {err}");
        std::process::exit(1);
    }
}

fn run() -> Result<(), String> {
    let args: Vec<String> = env::args().collect();
    let (app, invoked, rest) = select(&args)?;
    let (os, arch) = machine();
    let pin = pin(app, os, arch)?;
    let data = data_home()?;
    let path = match decide(&data, app, pin.sha256) {
        Action::Run(path) => path,
        Action::Fetch { cache } => fetch(app, pin, &cache)?,
    };
    let mut cmd = Command::new(&path);
    if app.arg0 {
        cmd.arg0(&invoked);
    }
    let err = cmd.args(&rest).exec();
    Err(format!("hoststub: {err}"))
}

fn fetch(app: &App, pin: &hoststub::Pin, cache: &Path) -> Result<PathBuf, String> {
    let parent = cache
        .parent()
        .ok_or_else(|| "hoststub: the cache has no parent".to_string())?;
    fs::create_dir_all(parent).map_err(|err| err.to_string())?;
    let part = parent.join(format!(".{}.partial", app.version));
    let _ = fs::remove_file(&part);
    let mut bar = Bar::new(app.name, app.version, pin.size);
    get::download(pin.url, &part, pin.size, &mut |got| bar.bump(got))?;
    bar.finish();
    eprintln!("check sha256");
    let got = sha256_file(&part)?;
    if got != pin.sha256 {
        let _ = fs::remove_file(&part);
        return Err(format!("hoststub: sha256 {got}"));
    }
    eprintln!("extract");
    let _ = fs::remove_dir_all(cache);
    fs::create_dir_all(cache).map_err(|err| err.to_string())?;
    let placed = match app.pack {
        Pack::TarGz { strip } => extract(&part, cache, strip, app.exec),
        Pack::Gzip => inflate(&part, &cache.join(app.exec), pin.plain),
    };
    if let Err(err) = placed {
        let _ = fs::remove_dir_all(cache);
        let _ = fs::remove_file(&part);
        return Err(err);
    }
    let _ = fs::remove_file(&part);
    write_stamp(cache, pin.sha256, app.exec)?;
    Ok(cache.join(app.exec))
}

fn select(args: &[String]) -> Result<(&'static App, String, Vec<String>), String> {
    let file = args
        .first()
        .and_then(|arg| Path::new(arg).file_name())
        .and_then(|name| name.to_str())
        .unwrap_or("");
    if file == "hoststub" || file.is_empty() {
        let name = args
            .get(1)
            .ok_or_else(|| "hoststub: the app name is missing".to_string())?;
        let app = find(name)?;
        return Ok((app, name.clone(), args.get(2..).unwrap_or(&[]).to_vec()));
    }
    let app = find(file)?;
    Ok((app, file.to_string(), args.get(1..).unwrap_or(&[]).to_vec()))
}

fn data_home() -> Result<PathBuf, String> {
    if let Some(value) = env::var_os("XDG_DATA_HOME") {
        if !value.is_empty() {
            return Ok(PathBuf::from(value));
        }
    }
    let home = env::var_os("HOME").ok_or_else(|| "hoststub: HOME is empty".to_string())?;
    Ok(PathBuf::from(home).join(".local").join("share"))
}

struct Bar {
    name: String,
    version: String,
    total: u64,
    next: u64,
    shown: u64,
    tty: bool,
}

impl Bar {
    fn new(name: &str, version: &str, total: u64) -> Self {
        Self {
            name: name.to_string(),
            version: version.to_string(),
            total,
            next: 0,
            shown: 0,
            tty: io::stderr().is_terminal(),
        }
    }

    fn bump(&mut self, got: u64) {
        if got < self.shown {
            self.next = 0;
        }
        self.shown = got;
        if got < self.next && got < self.total {
            return;
        }
        let step = (self.total / 200).max(256 * 1024);
        self.next = got.saturating_add(step);
        let line = progress_line(&self.name, &self.version, got.min(self.total), self.total);
        if self.tty {
            eprint!("\r{line}");
            let _ = io::stderr().flush();
        }
    }

    fn finish(&mut self) {
        let line = progress_line(&self.name, &self.version, self.total, self.total);
        if self.tty {
            eprintln!("\r{line}");
        } else {
            eprintln!("{line}");
        }
    }
}
