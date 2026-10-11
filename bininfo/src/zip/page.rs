/*
---
why: This file writes one zip archive as one text page.
uses: bininfo
how: bininfo calls this file after it reads the central directory.
This file does not read zip bytes.
---
*/

//! One page for one zip archive.
//! Each line is `key: value  # note`. This file does not read zip bytes.

use super::{method_name, Archive, Member};
use crate::style::{self, Json};
use std::fmt::Write;

pub fn render(file: &str, archive: &Archive, color: bool) -> String {
    let mut out = String::new();
    style::field(
        &mut out,
        color,
        0,
        "file",
        file,
        "Path of this file.",
        style::WHITE,
        false,
    );
    style::field(
        &mut out,
        color,
        0,
        "format",
        "zip",
        "A zip archive has no start claim.",
        style::FG,
        true,
    );
    style::blank(&mut out);
    type_rung(&mut out, archive, color);
    style::blank(&mut out);
    members_rung(&mut out, archive, color);
    out
}

pub fn render_json(file: &str, archive: &Archive) -> String {
    let mut out = Json::new();
    out.str("file", file);
    out.str("format", "zip");
    out.str("profile", archive.profile);
    out.flag("zip64", archive.zip64);
    out.flag("spanned", archive.spanned);
    out.flag("encrypted", archive.encrypted);
    out.u64("count", archive.count);
    out.u64("listed", archive.listed as u64);
    out.u64("comment", u64::from(archive.comment));
    let rows: Vec<String> = archive.members.iter().map(member_json).collect();
    out.raw_array("members", &rows);
    out.finish()
}

fn type_rung(out: &mut String, archive: &Archive, color: bool) {
    style::rung(
        out,
        color,
        "type",
        "Profile, zip64, and the central-directory count.",
        style::GREEN,
        true,
    );
    style::field(
        out,
        color,
        2,
        "profile",
        archive.profile,
        "apk, ipa, jar, ooxml, epub, wheel, or zip.",
        style::FG,
        true,
    );
    style::field(
        out,
        color,
        2,
        "zip64",
        bool_word(archive.zip64),
        "True when the end record is zip64.",
        flag_tone(archive.zip64),
        archive.zip64,
    );
    style::field(
        out,
        color,
        2,
        "spanned",
        bool_word(archive.spanned),
        "True when the archive uses more than one disk.",
        flag_tone(archive.spanned),
        false,
    );
    style::field(
        out,
        color,
        2,
        "encrypted",
        bool_word(archive.encrypted),
        "True when a member has the encryption flag.",
        flag_tone(archive.encrypted),
        false,
    );
    style::field(
        out,
        color,
        2,
        "count",
        &archive.count.to_string(),
        "Rows in the central directory.",
        style::FG,
        false,
    );
    style::field(
        out,
        color,
        2,
        "listed",
        &archive.listed.to_string(),
        "Rows kept in this page.",
        style::FG,
        false,
    );
    style::field(
        out,
        color,
        2,
        "comment",
        &archive.comment.to_string(),
        "Byte length of the end comment.",
        style::FG,
        false,
    );
}

fn members_rung(out: &mut String, archive: &Archive, color: bool) {
    style::rung(
        out,
        color,
        "members",
        "One row is one central-directory entry.",
        style::FG,
        true,
    );
    if archive.members.is_empty() {
        style::field(
            out,
            color,
            2,
            "rows",
            "[]",
            "No member in the kept rows.",
            style::LABEL,
            false,
        );
        return;
    }
    for member in &archive.members {
        write_member(out, member, color);
    }
}

fn write_member(out: &mut String, member: &Member, color: bool) {
    let shown_plain = style::yaml_scalar(&member.name);
    let shown = style::file_text(color, &member.name);
    let dash = style::paint(color, "-", style::LABEL, false);
    let prefix = format!("  {dash} {shown}");
    let plain_len = 4 + shown_plain.chars().count();
    if member.encrypted {
        style::align_note(out, color, &prefix, plain_len, "Encrypted member.");
    } else {
        let _ = writeln!(out, "{prefix}");
    }
    let method = style::tagged(method_name(member.method), u64::from(member.method));
    style::field(
        out,
        color,
        4,
        "method",
        &method,
        "Compression method.",
        style::FG,
        false,
    );
    let (fg, bold) = style::number_tone(member.size, style::FG, false);
    style::field(
        out,
        color,
        4,
        "size",
        &member.size.to_string(),
        "Uncompressed size.",
        fg,
        bold,
    );
    let (fg, bold) = style::number_tone(member.compressed, style::FG, false);
    style::field(
        out,
        color,
        4,
        "stored",
        &member.compressed.to_string(),
        "Stored size.",
        fg,
        bold,
    );
    let crc = format!("CRC {}.", style::hex(u64::from(member.crc)));
    style::field(
        out,
        color,
        4,
        "offset",
        &member.offset.to_string(),
        &crc,
        style::FG,
        false,
    );
}

fn member_json(member: &Member) -> String {
    let mut out = Json::new();
    out.str("name", &member.name);
    out.opt("method", method_name(member.method));
    out.u64("method_id", u64::from(member.method));
    out.u64("size", member.size);
    out.u64("stored", member.compressed);
    out.u64("crc", u64::from(member.crc));
    out.u64("offset", member.offset);
    out.flag("encrypted", member.encrypted);
    out.finish_inline()
}

fn bool_word(value: bool) -> &'static str {
    if value {
        "true"
    } else {
        "false"
    }
}

fn flag_tone(value: bool) -> style::Rgb {
    if value {
        style::PINK
    } else {
        style::LABEL
    }
}
