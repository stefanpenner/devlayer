/*
---
why: This file draws the folded page in the terminal.
uses: bininfo
how: The bininfo command calls this file when stdout is a terminal.
This file reads keys and draws the page.
---
*/

//! Draw the folded page. Keys move, open, close, and search.

use super::fold::{self, View};
use crossterm::cursor::{Hide, MoveTo, Show};
use crossterm::event::{self, Event, KeyCode, KeyEventKind, KeyModifiers};
use crossterm::style::{Attribute, Print, SetAttribute};
use crossterm::terminal::{
    self, Clear, ClearType, DisableLineWrap, EnableLineWrap, EnterAlternateScreen,
    LeaveAlternateScreen,
};
use crossterm::{execute, queue};
use std::io::{self, Write};

pub(crate) fn run(page: &str) -> io::Result<()> {
    let mut view = View::spine(page);
    let mut stdout = io::stdout();
    terminal::enable_raw_mode()?;
    let _guard = Restorer;
    execute!(stdout, EnterAlternateScreen, Hide, DisableLineWrap)?;
    let mut typed = String::new();
    let mut search = false;
    loop {
        draw(&mut stdout, &view, search.then_some(typed.as_str()))?;
        if let Event::Key(key) = event::read()? {
            if key.kind != KeyEventKind::Press {
                continue;
            }
            if key.modifiers.contains(KeyModifiers::CONTROL) && key.code == KeyCode::Char('c') {
                break;
            }
            if search {
                match key.code {
                    KeyCode::Enter => {
                        if typed.is_empty() {
                            view.find("");
                        } else {
                            view.find(&typed);
                        }
                        search = false;
                    }
                    KeyCode::Esc => search = false,
                    KeyCode::Backspace => {
                        typed.pop();
                    }
                    KeyCode::Char(ch) => typed.push(ch),
                    _ => {}
                }
                continue;
            }
            match key.code {
                KeyCode::Char('q') | KeyCode::Esc => break,
                KeyCode::Char('j') | KeyCode::Down => view.down(),
                KeyCode::Char('k') | KeyCode::Up => view.up(),
                KeyCode::Enter | KeyCode::Right | KeyCode::Char('l') => view.open(),
                KeyCode::Char('h') | KeyCode::Left | KeyCode::Backspace => view.close(),
                KeyCode::Char('/') => {
                    typed.clear();
                    search = true;
                }
                KeyCode::Char('n') => view.next(),
                _ => {}
            }
        }
    }
    Ok(())
}

fn draw(out: &mut io::Stdout, view: &View, query: Option<&str>) -> io::Result<()> {
    let (cols, rows) = terminal::size()?;
    let width = cols as usize;
    let lines = view.slots_text();
    let cursor = view.cursor_slot();
    let body = rows.saturating_sub(1) as usize;
    let start = window_start(cursor, lines.len(), body);
    queue!(out, MoveTo(0, 0), Clear(ClearType::All))?;
    for (row, line) in lines.iter().skip(start).take(body).enumerate() {
        queue!(out, MoveTo(0, row as u16))?;
        if start + row == cursor {
            let plain = fold::strip_ansi(line);
            queue!(
                out,
                SetAttribute(Attribute::Reverse),
                Print(fit(&plain, width)),
                SetAttribute(Attribute::Reset)
            )?;
        } else {
            queue!(out, Print(fit(line, width)), Print("\x1b[0m"))?;
        }
    }
    let status = match query {
        Some(text) => format!("/{text}"),
        None => "j/k move    enter open    h close    / search    n next    q quit".to_string(),
    };
    queue!(
        out,
        MoveTo(0, rows.saturating_sub(1)),
        SetAttribute(Attribute::Reverse),
        Print(fit(&status, width)),
        SetAttribute(Attribute::Reset)
    )?;
    out.flush()
}

fn window_start(cursor: usize, len: usize, height: usize) -> usize {
    if height == 0 || len <= height {
        return 0;
    }
    let half = height / 2;
    cursor.saturating_sub(half).min(len - height)
}

fn fit(line: &str, width: usize) -> String {
    if width == 0 {
        return String::new();
    }
    let mut out = String::new();
    let mut seen = 0usize;
    let mut chars = line.chars().peekable();
    while let Some(ch) = chars.next() {
        if ch == '\u{1b}' && chars.peek() == Some(&'[') {
            out.push(ch);
            if let Some(next) = chars.next() {
                out.push(next);
            }
            for next in chars.by_ref() {
                out.push(next);
                if next.is_ascii_alphabetic() {
                    break;
                }
            }
            continue;
        }
        if seen >= width {
            break;
        }
        out.push(ch);
        seen += 1;
    }
    out
}

struct Restorer;

impl Drop for Restorer {
    fn drop(&mut self) {
        let mut stdout = io::stdout();
        let _ = execute!(
            stdout,
            EnableLineWrap,
            Show,
            LeaveAlternateScreen,
            SetAttribute(Attribute::Reset)
        );
        let _ = terminal::disable_raw_mode();
    }
}
