//! Mach-O header and how the file starts.

pub mod header;
pub mod json;
pub mod load;
pub mod page;
pub(crate) mod symbols;
mod words;

pub(crate) use words::{cpu_name, universal_word};
