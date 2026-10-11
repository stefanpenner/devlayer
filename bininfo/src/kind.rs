//! The start kind shared by ELF, Mach-O, and PE.
//! Each format chooses the kind. This file only names it.

/// How a file starts. The same word means the same fact in each format.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Kind {
    Static,
    Dynamic,
    DynamicPie,
    StaticPie,
    Shared,
    Rel,
    Other,
}

pub fn name(kind: Kind) -> &'static str {
    match kind {
        Kind::Static => "static",
        Kind::Dynamic => "dynamic",
        Kind::DynamicPie => "dynamic-pie",
        Kind::StaticPie => "static-pie",
        Kind::Shared => "shared",
        Kind::Rel => "rel",
        Kind::Other => "other",
    }
}

/// The kind for one policy word.
pub fn parse_name(word: &str) -> Result<Kind, String> {
    match word {
        "static" => Ok(Kind::Static),
        "dynamic" => Ok(Kind::Dynamic),
        "dynamic-pie" => Ok(Kind::DynamicPie),
        "static-pie" => Ok(Kind::StaticPie),
        "shared" => Ok(Kind::Shared),
        "rel" => Ok(Kind::Rel),
        "other" => Ok(Kind::Other),
        _ => Err(format!("kind: found {word}")),
    }
}
