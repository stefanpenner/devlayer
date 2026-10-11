/*
---
why: This file is the bininfo command.
uses: bininfo
how: The command reads arguments and one file.
A terminal folds the page.
A pipe prints the full page.
It calls the bininfo library to print a page or check a claim.
---
*/

//! Read an ELF, Mach-O, PE, zip, dmg, or tiff file and print it.
//! A terminal folds the page. A pipe prints the full page.
//! `--check` asks whether the header names no other file.
//! `--policy` asks whether each recorded library is in that policy.

mod fold;
mod tui;

use std::env;
use std::fs::{self, File};
use std::io::{self, IsTerminal, Read};
use std::process::ExitCode;

const USAGE: &str = "\
usage: bininfo [--json] [--symbols] [--check] [--expect <claim>] <file>
usage: bininfo [--json] --claims <file>
usage: bininfo --policy <file> <path>
A terminal folds the page. A pipe prints the full page.
--policy checks one file or every file under a directory.";

fn main() -> ExitCode {
    match run(env::args().skip(1)) {
        Ok(code) => code,
        Err(err) => {
            eprintln!("{err}");
            ExitCode::from(1)
        }
    }
}

enum Job {
    Show {
        path: String,
        json: bool,
        symbols: bool,
    },
    Check {
        path: String,
        json: bool,
        expect: Vec<String>,
    },
    Claims {
        path: String,
        json: bool,
    },
    Policy {
        policy: String,
        paths: Vec<String>,
    },
}

fn run(args: impl Iterator<Item = String>) -> Result<ExitCode, String> {
    match args_of(args)? {
        Job::Show {
            path,
            json,
            symbols,
        } => {
            let bytes = read_file(&path)?;
            let file = bininfo::open(&bytes, symbols)?;
            let text = bininfo::render(&path, &file, json, color_stdout());
            if fold::interactive(io::stdout().is_terminal(), json) {
                tui::run(&text).map_err(|err| err.to_string())?;
            } else {
                print!("{text}");
            }
            Ok(ExitCode::SUCCESS)
        }
        Job::Check { path, json, expect } => {
            let tokens = if expect.is_empty() {
                vec!["hermetic".to_string()]
            } else {
                expect
            };
            let claims = bininfo::claim::parse(&tokens)?;
            check_one(&path, &claims, json)
        }
        Job::Claims { path, json } => check_file(&path, json),
        Job::Policy { policy, paths } => {
            let text = fs::read_to_string(&policy)
                .map_err(|err| format!("cannot read {policy}: {err}"))?;
            let rules = bininfo::policy::parse(&text).map_err(|err| format!("{policy}: {err}"))?;
            let mut failed = false;
            for path in &paths {
                if let Err(err) = bininfo::osload::check_path(std::path::Path::new(path), &rules) {
                    eprintln!("{err}");
                    failed = true;
                }
            }
            if failed {
                Ok(ExitCode::from(1))
            } else {
                Ok(ExitCode::SUCCESS)
            }
        }
    }
}

fn check_one(path: &str, claims: &[bininfo::claim::Claim], json: bool) -> Result<ExitCode, String> {
    let bytes = read_file(path)?;
    let file = bininfo::open(&bytes, false)?;
    let facts = bininfo::facts(path, &file, &bininfo::current_host())?;
    let misses = bininfo::claim::judge(&facts, claims);
    print_report(&facts, &misses, json);
    Ok(code_of(&misses))
}

fn check_file(path: &str, json: bool) -> Result<ExitCode, String> {
    let text = fs::read_to_string(path).map_err(|err| format!("cannot read {path}: {err}"))?;
    let mut code = ExitCode::SUCCESS;
    let mut count = 0usize;
    let mut first = true;
    for (index, raw) in text.lines().enumerate() {
        let line = raw.trim_start_matches('\u{feff}').trim();
        if line.is_empty()
            || line.starts_with('#')
            || line.chars().all(|c| c == '\u{200b}' || c == '\u{feff}')
        {
            continue;
        }
        count += 1;
        let mut parts = line.split_whitespace();
        let file = parts.next().expect("a trimmed line has a token");
        let tokens: Vec<String> = parts.map(str::to_string).collect();
        let claims =
            bininfo::claim::parse(&tokens).map_err(|err| format!("{path}:{}: {err}", index + 1))?;
        let claims = if claims.is_empty() {
            bininfo::claim::parse(&["hermetic".to_string()])?
        } else {
            claims
        };
        let bytes = match read_file(file) {
            Ok(bytes) => bytes,
            Err(err) => {
                eprintln!("{err}");
                code = ExitCode::from(1);
                continue;
            }
        };
        let opened = match bininfo::open(&bytes, false) {
            Ok(opened) => opened,
            Err(err) => {
                eprintln!("{file}: {err}");
                code = ExitCode::from(1);
                continue;
            }
        };
        let facts = match bininfo::facts(file, &opened, &bininfo::current_host()) {
            Ok(facts) => facts,
            Err(err) => {
                eprintln!("{file}: {err}");
                code = ExitCode::from(1);
                continue;
            }
        };
        let misses = bininfo::claim::judge(&facts, &claims);
        if !misses.is_empty() {
            code = ExitCode::from(1);
        }
        if !json && !first {
            println!("---");
        }
        first = false;
        print_report(&facts, &misses, json);
    }
    if count == 0 {
        return Err(format!("{path}: no file"));
    }
    Ok(code)
}

fn print_report(facts: &bininfo::claim::Facts, misses: &[bininfo::claim::Miss], json: bool) {
    if json {
        print!("{}", bininfo::claim::report_json(facts, misses));
    } else {
        print!("{}", bininfo::claim::report(facts, misses, color_stdout()));
    }
}

fn code_of(misses: &[bininfo::claim::Miss]) -> ExitCode {
    if misses.is_empty() {
        ExitCode::SUCCESS
    } else {
        ExitCode::from(1)
    }
}

fn args_of(args: impl Iterator<Item = String>) -> Result<Job, String> {
    let args: Vec<String> = args.collect();
    let mut json = false;
    let mut symbols = false;
    let mut check = false;
    let mut claims = None;
    let mut expect = Vec::new();
    let mut policy = None;
    let mut paths = Vec::new();
    let mut index = 0;
    while index < args.len() {
        let arg = &args[index];
        if claims.is_some() || (policy.is_none() && !paths.is_empty()) {
            return Err(USAGE.into());
        }
        if arg == "--json" {
            json = true;
        } else if arg == "--symbols" {
            symbols = true;
        } else if arg == "--check" {
            check = true;
        } else if arg == "--policy" {
            index += 1;
            let Some(file) = args.get(index) else {
                return Err(USAGE.into());
            };
            if file.starts_with('-') || policy.is_some() {
                return Err(USAGE.into());
            }
            policy = Some(file.clone());
        } else if arg == "--claims" {
            index += 1;
            let Some(file) = args.get(index) else {
                return Err(USAGE.into());
            };
            if file.starts_with('-') || claims.is_some() {
                return Err(USAGE.into());
            }
            claims = Some(file.clone());
        } else if arg == "--expect" {
            index += 1;
            let Some(token) = args.get(index) else {
                return Err(USAGE.into());
            };
            if token.starts_with('-') {
                return Err(USAGE.into());
            }
            expect.push(token.clone());
        } else if arg.starts_with('-') {
            return Err(USAGE.into());
        } else if policy.is_some() || paths.is_empty() {
            paths.push(arg.clone());
        } else {
            return Err(USAGE.into());
        }
        index += 1;
    }
    if let Some(file) = claims {
        if policy.is_some() || !paths.is_empty() || check || symbols || !expect.is_empty() {
            return Err(USAGE.into());
        }
        return Ok(Job::Claims { path: file, json });
    }
    if let Some(file) = policy {
        if paths.is_empty() || check || symbols || json || !expect.is_empty() {
            return Err(USAGE.into());
        }
        return Ok(Job::Policy {
            policy: file,
            paths,
        });
    }
    let Some(path) = paths.pop() else {
        return Err(USAGE.into());
    };
    if !paths.is_empty() {
        return Err(USAGE.into());
    }
    if check || !expect.is_empty() {
        return Ok(Job::Check { path, json, expect });
    }
    Ok(Job::Show {
        path,
        json,
        symbols,
    })
}

fn color_stdout() -> bool {
    io::stdout().is_terminal() && env::var_os("NO_COLOR").is_none()
}

fn read_file(path: &str) -> Result<Vec<u8>, String> {
    let mut file = File::open(path).map_err(|err| format!("cannot read {path}: {err}"))?;
    let mut bytes = Vec::new();
    file.read_to_end(&mut bytes)
        .map_err(|err| format!("cannot read {path}: {err}"))?;
    Ok(bytes)
}
