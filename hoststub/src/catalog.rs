//! One host app is a name, a version, one archive pin per machine, and the host libraries it still needs.

/// Pin is one download for one OS and one CPU.
#[derive(Clone, Copy)]
pub struct Pin {
    pub os: &'static str,
    pub arch: &'static str,
    pub url: &'static str,
    pub sha256: &'static str,
    /// size is the download cap.
    pub size: u64,
    /// plain is the size after gzip. A tar.gz pin leaves this at 0.
    pub plain: u64,
}

/// Pack is how the pin bytes become the program.
#[derive(Clone, Copy)]
pub enum Pack {
    /// A tar.gz. `strip` drops leading path parts.
    TarGz { strip: usize },
    /// One gzip file. The bytes after gzip are the program.
    Gzip,
}

/// HostLib is one library the downloaded program still needs.
pub struct HostLib {
    pub name: &'static str,
    pub why: &'static str,
}

/// App is one program the stub can run.
pub struct App {
    pub name: &'static str,
    pub aliases: &'static [&'static str],
    pub version: &'static str,
    pub exec: &'static str,
    pub pack: Pack,
    /// arg0 keeps the name the user typed.
    pub arg0: bool,
    /// install runs the program setup once, then runs the installed copy.
    pub install: bool,
    /// links are libraries the downloaded program still needs.
    /// An empty slice means this row does not record them yet.
    pub links: &'static [HostLib],
    pub pins: &'static [Pin],
}

const TERMINAL_BROWSER: &[Pin] = &[
    Pin {
        os: "darwin",
        arch: "arm64",
        url: "https://terminal-browser.sh/install/dl/stable/v0.13.4/terminal-browser-darwin-arm64.tar.gz",
        sha256: "f017230c78c60a07ef4451a1eb0a92727f0b955a8fcd87aec358910c5d0c03c7",
        size: 141_704_278,
        plain: 0,
    },
    Pin {
        os: "darwin",
        arch: "x64",
        url: "https://terminal-browser.sh/install/dl/stable/v0.13.4/terminal-browser-darwin-x64.tar.gz",
        sha256: "01bc6991bad122f42e4f2a5164a198d8384b944c036de112078fd51f58dc67ed",
        size: 149_889_617,
        plain: 0,
    },
    Pin {
        os: "linux",
        arch: "arm64",
        url: "https://terminal-browser.sh/install/dl/stable/v0.13.4/terminal-browser-linux-arm64.tar.gz",
        sha256: "0cf567d8218995a24fb6ce4b06c07ccf58a8c517906058087355f1e2fad1969f",
        size: 138_044_592,
        plain: 0,
    },
    Pin {
        os: "linux",
        arch: "x64",
        url: "https://terminal-browser.sh/install/dl/stable/v0.13.4/terminal-browser-linux-x64.tar.gz",
        sha256: "6277daaabab16711ab3f1961cdffad9efac5e70ac55d5076e2c86496d649d3a4",
        size: 136_646_100,
        plain: 0,
    },
];

const GROK: &[Pin] = &[
    Pin {
        os: "darwin",
        arch: "arm64",
        url: "https://x.ai/cli/grok-1.0.52-macos-aarch64.gz",
        sha256: "6e37ef32bfc99173644284566fd8243f3ee573a3ded373a07f8d3782e6c3a95f",
        size: 69_718_933,
        plain: 172_237_072,
    },
    Pin {
        os: "darwin",
        arch: "x64",
        url: "https://x.ai/cli/grok-1.0.52-macos-x86_64.gz",
        sha256: "886182c4df90b1f51bb5d349dc31496c6265773c8aeac0c36d018e3f500be7a8",
        size: 75_142_002,
        plain: 185_857_856,
    },
    Pin {
        os: "linux",
        arch: "arm64",
        url: "https://x.ai/cli/grok-1.0.52-linux-aarch64.gz",
        sha256: "f56c5210eabb3dba1977715454c6c2466ae74dea9a41d5710a2d333160492ef5",
        size: 69_055_281,
        plain: 160_670_952,
    },
    Pin {
        os: "linux",
        arch: "x64",
        url: "https://x.ai/cli/grok-1.0.52-linux-x86_64.gz",
        sha256: "28234f82ce7f24994d6902d102b8c9b135e2e8c75ee492f2d944a2cf0b081c79",
        size: 73_985_594,
        plain: 188_326_752,
    },
];

const TERMIUM: &[Pin] = &[
    Pin {
        os: "darwin",
        arch: "arm64",
        url:
            "https://github.com/codr1/termium/releases/download/v0.2.4/termium-darwin-arm64.tar.gz",
        sha256: "1230a1956163bf66ed29de551434245fd8bc7f24e102c46517be4d5b0364a64f",
        size: 50_684_482,
        plain: 0,
    },
    Pin {
        os: "darwin",
        arch: "x64",
        url:
            "https://github.com/codr1/termium/releases/download/v0.2.4/termium-darwin-amd64.tar.gz",
        sha256: "f73a92f9904570fe1913469e6cc478d85af473b30491481d7ae812d8e306bd1b",
        size: 52_225_792,
        plain: 0,
    },
    Pin {
        os: "linux",
        arch: "x64",
        url: "https://github.com/codr1/termium/releases/download/v0.2.4/termium-linux-amd64.tar.gz",
        sha256: "1f4d9ef2b19533d1429be98df6020af2f149ef20e685db77e7402f7544fad175",
        size: 179_532_692,
        plain: 0,
    },
];

const SIMSERVE: &[Pin] = &[Pin {
    os: "darwin",
    arch: "arm64",
    url: "https://github.com/stefanpenner/lcc.live/releases/download/simserve-0.1.0/simserve-darwin-arm64.gz",
    sha256: "ef242a999e103551408e8d579424e278964ec96badb71ff46062807ef93f8378",
    size: 298_822,
    plain: 723_488,
}];

/// JPEG_CAN_LEAVE is the reason for the three frameworks JPEG encode still uses.
const JPEG_CAN_LEAVE: &str = "JPEG encode uses this framework. A later pin can drop it.";

/// SIMSERVE_LINKS records `otool -L` plus the two frameworks the program opens by name.
const SIMSERVE_LINKS: &[HostLib] = &[
    HostLib {
        name: "/usr/lib/libobjc.A.dylib",
        why: "The program sends Objective-C messages.",
    },
    HostLib {
        name: "/System/Library/Frameworks/IOSurface.framework/Versions/A/IOSurface",
        why: "The program reads the phone framebuffer.",
    },
    HostLib {
        name: "/System/Library/Frameworks/CoreFoundation.framework/Versions/A/CoreFoundation",
        why: JPEG_CAN_LEAVE,
    },
    HostLib {
        name: "/System/Library/Frameworks/CoreGraphics.framework/Versions/A/CoreGraphics",
        why: JPEG_CAN_LEAVE,
    },
    HostLib {
        name: "/System/Library/Frameworks/ImageIO.framework/Versions/A/ImageIO",
        why: JPEG_CAN_LEAVE,
    },
    HostLib {
        name: "/usr/lib/libSystem.B.dylib",
        why: "macOS has no static libc. libc, libm, and pthread are in this file.",
    },
    HostLib {
        name: "/Library/Developer/PrivateFrameworks/CoreSimulator.framework/CoreSimulator",
        why: "The program opens this framework by name. It talks to the booted phone. It stays on the Mac.",
    },
    HostLib {
        name: "SimulatorKit.framework/SimulatorKit",
        why: "The program opens this framework by name. Touch uses it. It stays on the Mac.",
    },
];

/// APPS is the catalog. The file name selects one row.
pub static APPS: &[App] = &[
    App {
        name: "terminal-browser",
        aliases: &[],
        version: "v0.13.4",
        exec: "bin/terminal-browser",
        pack: Pack::TarGz { strip: 1 },
        arg0: false,
        install: false,
        links: &[],
        pins: TERMINAL_BROWSER,
    },
    App {
        name: "grok",
        aliases: &["agent"],
        version: "1.0.52",
        exec: "grok",
        pack: Pack::Gzip,
        arg0: true,
        install: false,
        links: &[],
        pins: GROK,
    },
    App {
        name: "termium",
        aliases: &[],
        version: "v0.2.4",
        exec: "bin/termium",
        pack: Pack::TarGz { strip: 0 },
        arg0: true,
        install: true,
        links: &[],
        pins: TERMIUM,
    },
    App {
        name: "simserve",
        aliases: &[],
        version: "0.1.0",
        exec: "simserve",
        pack: Pack::Gzip,
        arg0: false,
        install: false,
        links: SIMSERVE_LINKS,
        pins: SIMSERVE,
    },
];

/// find returns the app with this name or one of its aliases.
pub fn find(name: &str) -> Result<&'static App, String> {
    APPS.iter()
        .find(|app| app.name == name || app.aliases.contains(&name))
        .ok_or_else(|| format!("hoststub: no app named {name}"))
}

/// pin returns the archive for one OS and one CPU.
pub fn pin(app: &App, os: &str, arch: &str) -> Result<&'static Pin, String> {
    app.pins
        .iter()
        .find(|pin| pin.os == os && pin.arch == arch)
        .ok_or_else(|| format!("hoststub: {0} has no pin for {os}/{arch}", app.name))
}

/// machine maps the Rust consts onto the pin names.
pub fn machine() -> (&'static str, &'static str) {
    let os = match std::env::consts::OS {
        "macos" => "darwin",
        other => other,
    };
    let arch = match std::env::consts::ARCH {
        "aarch64" => "arm64",
        "x86_64" => "x64",
        other => other,
    };
    (os, arch)
}
