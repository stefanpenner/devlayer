/*
---
why: This file writes one disk image as one text page.
uses: bininfo
how: bininfo calls this file after it reads the koly trailer.
This file does not read dmg bytes.
---
*/

//! One page for one UDIF disk image.
//! Each line is `key: value  # note`. This file does not read dmg bytes.

use super::{checksum_name, fourcc, variant_name, Image};
use crate::style::{self, Json};

pub fn render(file: &str, image: &Image, color: bool) -> String {
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
        "dmg",
        "An Apple disk image has no start claim.",
        style::FG,
        true,
    );
    style::blank(&mut out);
    type_rung(&mut out, image, color);
    style::blank(&mut out);
    map_rung(&mut out, image, color);
    style::blank(&mut out);
    style::list_field(
        &mut out,
        color,
        0,
        "names",
        &image.names,
        "Name values from the resource plist.",
        style::FG,
        false,
    );
    out
}

pub fn render_json(file: &str, image: &Image) -> String {
    let mut out = Json::new();
    out.str("file", file);
    out.str("format", "dmg");
    out.str("variant", &fourcc(&image.variant));
    out.opt("variant_name", variant_name(&image.variant));
    out.u64("version", u64::from(image.version));
    out.u64("flags", u64::from(image.flags));
    out.u64("sectors", image.sectors);
    out.u64("segment", u64::from(image.segment));
    out.u64("segments", u64::from(image.segments));
    out.u64("data_offset", image.data_offset);
    out.u64("data_length", image.data_length);
    out.u64("xml_offset", image.xml_offset);
    out.u64("xml_length", image.xml_length);
    out.opt("checksum", checksum_name(image.checksum));
    out.u64("checksum_id", u64::from(image.checksum));
    out.strings("names", &image.names);
    out.finish()
}

fn type_rung(out: &mut String, image: &Image, color: bool) {
    style::rung(
        out,
        color,
        "type",
        "Variant, sector count, and checksum from the koly trailer.",
        style::GREEN,
        true,
    );
    let variant = match variant_name(&image.variant) {
        Some(word) => format!("{} ({word})", fourcc(&image.variant)),
        None => fourcc(&image.variant),
    };
    style::field(
        out,
        color,
        2,
        "variant",
        &variant,
        "Fourcc of the image variant.",
        style::FG,
        true,
    );
    style::field(
        out,
        color,
        2,
        "version",
        &image.version.to_string(),
        "UDIF trailer version.",
        style::FG,
        false,
    );
    style::field(
        out,
        color,
        2,
        "sectors",
        &image.sectors.to_string(),
        "512-byte sector count.",
        style::FG,
        false,
    );
    style::field(
        out,
        color,
        2,
        "segment",
        &format!("{} of {}", image.segment, image.segments),
        "Segment index and segment count.",
        style::FG,
        false,
    );
    let checksum = style::tagged(checksum_name(image.checksum), u64::from(image.checksum));
    style::field(
        out,
        color,
        2,
        "checksum",
        &checksum,
        "Master checksum kind.",
        style::FG,
        false,
    );
}

fn map_rung(out: &mut String, image: &Image, color: bool) {
    style::rung(
        out,
        color,
        "map",
        "Data fork and resource plist from the koly trailer.",
        style::BLUE,
        true,
    );
    style::field(
        out,
        color,
        2,
        "flags",
        &style::hex(u64::from(image.flags)),
        "Trailer flags.",
        style::FG,
        false,
    );
    style::field(
        out,
        color,
        2,
        "data_offset",
        &image.data_offset.to_string(),
        "Data fork offset.",
        style::FG,
        false,
    );
    style::field(
        out,
        color,
        2,
        "data_length",
        &image.data_length.to_string(),
        "Data fork byte length.",
        style::FG,
        false,
    );
    style::field(
        out,
        color,
        2,
        "xml_offset",
        &image.xml_offset.to_string(),
        "Resource plist offset.",
        style::FG,
        false,
    );
    style::field(
        out,
        color,
        2,
        "xml_length",
        &image.xml_length.to_string(),
        "Resource plist byte length.",
        style::FG,
        false,
    );
}
