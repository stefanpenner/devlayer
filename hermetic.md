# Hermetic graph

This file is the portable hermetic build.
The sysroot is the output of that build.
A build target, a test target, and a run target use that sysroot.
`hermeticgraph` writes this file from `bazel query`.
Each layer is a topological band.
A child is an earlier input.
The query is the rules under `third_party`, `linux`, `macos`, `windows`, and `chain`, plus one direct dependency.
The generator omits `config_setting`, `constraint_value`, and `constraint_setting`.
A source file has no rule class.
A `@bazel_tools` label has no file in this repo.

## Layer 0

<a id="-versions.env"></a>

### `//:versions.env`

File [versions.env](versions.env).

<a id="macos-archive.sh"></a>

### `//macos:archive.sh`

File [macos/archive.sh](macos/archive.sh).

<a id="macos-host_make.sh"></a>

### `//macos:host_make.sh`

File [macos/host_make.sh](macos/host_make.sh).

<a id="macos-luajit.sh"></a>

### `//macos:luajit.sh`

File [macos/luajit.sh](macos/luajit.sh).

<a id="macos-toolbox.miniconfig"></a>

### `//macos:toolbox.miniconfig`

File [macos/toolbox.miniconfig](macos/toolbox.miniconfig).

<a id="macos-toolbox.sh"></a>

### `//macos:toolbox.sh`

File [macos/toolbox.sh](macos/toolbox.sh).

<a id="third_party-nvim-host_cc.sh"></a>

### `//third_party/nvim:host_cc.sh`

File [third_party/nvim/host_cc.sh](third_party/nvim/host_cc.sh).

<a id="third_party-nvim-host_luajit.sh"></a>

### `//third_party/nvim:host_luajit.sh`

File [third_party/nvim/host_luajit.sh](third_party/nvim/host_luajit.sh).

<a id="third_party-nvim-host_nlua0.sh"></a>

### `//third_party/nvim:host_nlua0.sh`

File [third_party/nvim/host_nlua0.sh](third_party/nvim/host_nlua0.sh).

<a id="third_party-nvim-host_nvim.sh"></a>

### `//third_party/nvim:host_nvim.sh`

File [third_party/nvim/host_nvim.sh](third_party/nvim/host_nvim.sh).

<a id="third_party-nvim-zig_ranlib.sh"></a>

### `//third_party/nvim:zig_ranlib.sh`

File [third_party/nvim/zig_ranlib.sh](third_party/nvim/zig_ranlib.sh).

<a id="tools-assemble-assemble"></a>

### `//tools/assemble:assemble`

Rule `go_binary`.
File [tools/assemble/BUILD.bazel](tools/assemble/BUILD.bazel#L15).

<a id="tools-packarchive-packarchive"></a>

### `//tools/packarchive:packarchive`

Rule `go_binary`.
File [tools/packarchive/BUILD.bazel](tools/packarchive/BUILD.bazel#L10).

<a id="age_darwin_arm64--age"></a>

### `@age_darwin_arm64//:age`

Pin [MODULE.bazel](MODULE.bazel#L127).

<a id="age_darwin_arm64--age-keygen"></a>

### `@age_darwin_arm64//:age-keygen`

Pin [MODULE.bazel](MODULE.bazel#L127).

<a id="age_linux_amd64--age"></a>

### `@age_linux_amd64//:age`

Pin [MODULE.bazel](MODULE.bazel#L125).

<a id="age_linux_amd64--age-keygen"></a>

### `@age_linux_amd64//:age-keygen`

Pin [MODULE.bazel](MODULE.bazel#L125).

<a id="age_linux_arm64--age"></a>

### `@age_linux_arm64//:age`

Pin [MODULE.bazel](MODULE.bazel#L126).

<a id="age_linux_arm64--age-keygen"></a>

### `@age_linux_arm64//:age-keygen`

Pin [MODULE.bazel](MODULE.bazel#L126).

<a id="bat_darwin_arm64--bat"></a>

### `@bat_darwin_arm64//:bat`

Pin [MODULE.bazel](MODULE.bazel#L111).

<a id="bat_extras--all"></a>

### `@bat_extras//:all`

Rule `filegroup`.
Pin [tools/extensions.bzl](tools/extensions.bzl#L516).

<a id="bat_linux_amd64--bat"></a>

### `@bat_linux_amd64//:bat`

Pin [MODULE.bazel](MODULE.bazel#L109).

<a id="bat_linux_arm64--bat"></a>

### `@bat_linux_arm64//:bat`

Pin [MODULE.bazel](MODULE.bazel#L110).

<a id="bazel_tools-tools-allowlists-function_transition_allowlist-function_transition_allowlist"></a>

### `@bazel_tools//tools/allowlists/function_transition_allowlist:function_transition_allowlist`


<a id="bazel_tools-tools-cpp-current_cc_toolchain"></a>

### `@bazel_tools//tools/cpp:current_cc_toolchain`

Rule `alias`.

<a id="bazel_tools-tools-cpp-toolchain_type"></a>

### `@bazel_tools//tools/cpp:toolchain_type`

Rule `toolchain_type`.

<a id="bazel_tools-tools-genrule-genrule-setup.sh"></a>

### `@bazel_tools//tools/genrule:genrule-setup.sh`


<a id="btop_src--all_srcs"></a>

### `@btop_src//:all_srcs`

Rule `filegroup`.
Pin [tools/extensions.bzl](tools/extensions.bzl#L125).

<a id="delta_darwin_arm64--delta"></a>

### `@delta_darwin_arm64//:delta`

Pin [MODULE.bazel](MODULE.bazel#L114).

<a id="delta_linux_amd64--delta"></a>

### `@delta_linux_amd64//:delta`

Pin [MODULE.bazel](MODULE.bazel#L112).

<a id="delta_linux_arm64--delta"></a>

### `@delta_linux_arm64//:delta`

Pin [MODULE.bazel](MODULE.bazel#L113).

<a id="direnv_darwin_arm64-file-direnv"></a>

### `@direnv_darwin_arm64//file:direnv`

Pin [MODULE.bazel](MODULE.bazel#L140).

<a id="direnv_linux_amd64-file-direnv"></a>

### `@direnv_linux_amd64//file:direnv`

Pin [MODULE.bazel](MODULE.bazel#L138).

<a id="direnv_linux_arm64-file-direnv"></a>

### `@direnv_linux_arm64//file:direnv`

Pin [MODULE.bazel](MODULE.bazel#L139).

<a id="dust_darwin_arm64--dust"></a>

### `@dust_darwin_arm64//:dust`

Pin [MODULE.bazel](MODULE.bazel#L117).

<a id="dust_linux_amd64--dust"></a>

### `@dust_linux_amd64//:dust`

Pin [MODULE.bazel](MODULE.bazel#L115).

<a id="dust_linux_arm64--dust"></a>

### `@dust_linux_arm64//:dust`

Pin [MODULE.bazel](MODULE.bazel#L116).

<a id="fast_syntax_highlighting--all"></a>

### `@fast_syntax_highlighting//:all`

Rule `filegroup`.
Pin [tools/extensions.bzl](tools/extensions.bzl#L551).

<a id="fd_darwin_arm64--fd"></a>

### `@fd_darwin_arm64//:fd`

Pin [MODULE.bazel](MODULE.bazel#L120).

<a id="fd_linux_amd64--fd"></a>

### `@fd_linux_amd64//:fd`

Pin [MODULE.bazel](MODULE.bazel#L118).

<a id="fd_linux_arm64--fd"></a>

### `@fd_linux_arm64//:fd`

Pin [MODULE.bazel](MODULE.bazel#L119).

<a id="fzf_darwin_arm64--fzf"></a>

### `@fzf_darwin_arm64//:fzf`

Pin [MODULE.bazel](MODULE.bazel#L130).

<a id="fzf_linux_amd64--fzf"></a>

### `@fzf_linux_amd64//:fzf`

Pin [MODULE.bazel](MODULE.bazel#L128).

<a id="fzf_linux_arm64--fzf"></a>

### `@fzf_linux_arm64//:fzf`

Pin [MODULE.bazel](MODULE.bazel#L129).

<a id="fzf_shell--shell_scripts"></a>

### `@fzf_shell//:shell_scripts`

Rule `filegroup`.
Pin [tools/extensions.bzl](tools/extensions.bzl#L525).

<a id="git_src--all_srcs"></a>

### `@git_src//:all_srcs`

Rule `filegroup`.
Pin [tools/extensions.bzl](tools/extensions.bzl#L184).

<a id="go_sdk_darwin_arm64--all"></a>

### `@go_sdk_darwin_arm64//:all`

Rule `filegroup`.
Pin [MODULE.bazel](MODULE.bazel#L147).

<a id="go_sdk_linux_amd64--all"></a>

### `@go_sdk_linux_amd64//:all`

Rule `filegroup`.
Pin [MODULE.bazel](MODULE.bazel#L145).

<a id="go_sdk_linux_arm64--all"></a>

### `@go_sdk_linux_arm64//:all`

Rule `filegroup`.
Pin [MODULE.bazel](MODULE.bazel#L146).

<a id="htop_src--all_srcs"></a>

### `@htop_src//:all_srcs`

Rule `filegroup`.
Pin [tools/extensions.bzl](tools/extensions.bzl#L147).

<a id="jq_darwin_arm64-file-jq"></a>

### `@jq_darwin_arm64//file:jq`

Pin [MODULE.bazel](MODULE.bazel#L143).

<a id="jq_linux_amd64-file-jq"></a>

### `@jq_linux_amd64//file:jq`

Pin [MODULE.bazel](MODULE.bazel#L141).

<a id="jq_linux_arm64-file-jq"></a>

### `@jq_linux_arm64//file:jq`

Pin [MODULE.bazel](MODULE.bazel#L142).

<a id="lazygit_darwin_arm64--lazygit"></a>

### `@lazygit_darwin_arm64//:lazygit`

Pin [MODULE.bazel](MODULE.bazel#L133).

<a id="lazygit_linux_amd64--lazygit"></a>

### `@lazygit_linux_amd64//:lazygit`

Pin [MODULE.bazel](MODULE.bazel#L131).

<a id="lazygit_linux_arm64--lazygit"></a>

### `@lazygit_linux_arm64//:lazygit`

Pin [MODULE.bazel](MODULE.bazel#L132).

<a id="macos_sdk--sdk.tar"></a>

### `@macos_sdk//:sdk.tar`

Pin [MODULE.bazel](MODULE.bazel#L40).

<a id="make_src--all_srcs"></a>

### `@make_src//:all_srcs`

Rule `filegroup`.
Pin [tools/extensions.bzl](tools/extensions.bzl#L154).

<a id="ncurses_src--all_srcs"></a>

### `@ncurses_src//:all_srcs`

Rule `filegroup`.
Pin [tools/extensions.bzl](tools/extensions.bzl#L177).

<a id="nvim_darwin_arm64--all"></a>

### `@nvim_darwin_arm64//:all`

Rule `filegroup`.
Pin [MODULE.bazel](MODULE.bazel#L150).

<a id="nvim_darwin_arm64--bin-nvim"></a>

### `@nvim_darwin_arm64//:bin/nvim`

Pin [MODULE.bazel](MODULE.bazel#L150).

<a id="nvim_libuv-file-archive.tar.gz"></a>

### `@nvim_libuv//file:archive.tar.gz`

Pin [MODULE.bazel](MODULE.bazel#L92).

<a id="nvim_linux_amd64--all"></a>

### `@nvim_linux_amd64//:all`

Rule `filegroup`.
Pin [MODULE.bazel](MODULE.bazel#L148).

<a id="nvim_linux_amd64--bin-nvim"></a>

### `@nvim_linux_amd64//:bin/nvim`

Pin [MODULE.bazel](MODULE.bazel#L148).

<a id="nvim_linux_arm64--all"></a>

### `@nvim_linux_arm64//:all`

Rule `filegroup`.
Pin [MODULE.bazel](MODULE.bazel#L149).

<a id="nvim_linux_arm64--bin-nvim"></a>

### `@nvim_linux_arm64//:bin/nvim`

Pin [MODULE.bazel](MODULE.bazel#L149).

<a id="nvim_lpeg-file-archive.tar.gz"></a>

### `@nvim_lpeg//file:archive.tar.gz`

Pin [MODULE.bazel](MODULE.bazel#L96).

<a id="nvim_lua_compat53-file-archive.tar.gz"></a>

### `@nvim_lua_compat53//file:archive.tar.gz`

Pin [MODULE.bazel](MODULE.bazel#L94).

<a id="nvim_luajit-file-archive.tar.gz"></a>

### `@nvim_luajit//file:archive.tar.gz`

Pin [MODULE.bazel](MODULE.bazel#L93).

<a id="nvim_luv-file-archive.tar.gz"></a>

### `@nvim_luv//file:archive.tar.gz`

Pin [MODULE.bazel](MODULE.bazel#L95).

<a id="nvim_src--all_srcs"></a>

### `@nvim_src//:all_srcs`

Rule `filegroup`.
Pin [tools/extensions.bzl](tools/extensions.bzl#L436).

<a id="nvim_treesitter-file-archive.tar.gz"></a>

### `@nvim_treesitter//file:archive.tar.gz`

Pin [MODULE.bazel](MODULE.bazel#L99).

<a id="nvim_treesitter_c-file-archive.tar.gz"></a>

### `@nvim_treesitter_c//file:archive.tar.gz`

Pin [MODULE.bazel](MODULE.bazel#L100).

<a id="nvim_treesitter_lua-file-archive.tar.gz"></a>

### `@nvim_treesitter_lua//file:archive.tar.gz`

Pin [MODULE.bazel](MODULE.bazel#L101).

<a id="nvim_treesitter_markdown-file-archive.tar.gz"></a>

### `@nvim_treesitter_markdown//file:archive.tar.gz`

Pin [MODULE.bazel](MODULE.bazel#L105).

<a id="nvim_treesitter_query-file-archive.tar.gz"></a>

### `@nvim_treesitter_query//file:archive.tar.gz`

Pin [MODULE.bazel](MODULE.bazel#L104).

<a id="nvim_treesitter_vim-file-archive.tar.gz"></a>

### `@nvim_treesitter_vim//file:archive.tar.gz`

Pin [MODULE.bazel](MODULE.bazel#L102).

<a id="nvim_treesitter_vimdoc-file-archive.tar.gz"></a>

### `@nvim_treesitter_vimdoc//file:archive.tar.gz`

Pin [MODULE.bazel](MODULE.bazel#L103).

<a id="nvim_unibilium-file-archive.tar.gz"></a>

### `@nvim_unibilium//file:archive.tar.gz`

Pin [MODULE.bazel](MODULE.bazel#L97).

<a id="nvim_utf8proc-file-archive.tar.gz"></a>

### `@nvim_utf8proc//file:archive.tar.gz`

Pin [MODULE.bazel](MODULE.bazel#L98).

<a id="platforms-os-macos"></a>

### `@platforms//os:macos`

Rule `alias`.
Pin [MODULE.bazel](MODULE.bazel#L47).

<a id="powerlevel10k--all"></a>

### `@powerlevel10k//:all`

Rule `filegroup`.
Pin [tools/extensions.bzl](tools/extensions.bzl#L569).

<a id="rg_darwin_arm64--rg"></a>

### `@rg_darwin_arm64//:rg`

Pin [MODULE.bazel](MODULE.bazel#L123).

<a id="rg_linux_amd64--rg"></a>

### `@rg_linux_amd64//:rg`

Pin [MODULE.bazel](MODULE.bazel#L121).

<a id="rg_linux_arm64--rg"></a>

### `@rg_linux_arm64//:rg`

Pin [MODULE.bazel](MODULE.bazel#L122).

<a id="rules_foreign_cc-foreign_cc-private-framework-platform_info"></a>

### `@rules_foreign_cc//foreign_cc/private/framework:platform_info`

Rule `_framework_platform_info`.
Pin [MODULE.bazel](MODULE.bazel#L66).

<a id="rules_foreign_cc-foreign_cc-private-framework-shell_toolchain"></a>

### `@rules_foreign_cc//foreign_cc/private/framework:shell_toolchain`

Rule `toolchain_type`.
Pin [MODULE.bazel](MODULE.bazel#L66).

<a id="rules_foreign_cc-toolchains-autoconf_toolchain"></a>

### `@rules_foreign_cc//toolchains:autoconf_toolchain`

Rule `toolchain_type`.
Pin [MODULE.bazel](MODULE.bazel#L66).

<a id="rules_foreign_cc-toolchains-automake_toolchain"></a>

### `@rules_foreign_cc//toolchains:automake_toolchain`

Rule `toolchain_type`.
Pin [MODULE.bazel](MODULE.bazel#L66).

<a id="rules_foreign_cc-toolchains-cmake_toolchain"></a>

### `@rules_foreign_cc//toolchains:cmake_toolchain`

Rule `toolchain_type`.
Pin [MODULE.bazel](MODULE.bazel#L66).

<a id="rules_foreign_cc-toolchains-m4_toolchain"></a>

### `@rules_foreign_cc//toolchains:m4_toolchain`

Rule `toolchain_type`.
Pin [MODULE.bazel](MODULE.bazel#L66).

<a id="rules_foreign_cc-toolchains-make_toolchain"></a>

### `@rules_foreign_cc//toolchains:make_toolchain`

Rule `toolchain_type`.
Pin [MODULE.bazel](MODULE.bazel#L66).

<a id="rules_foreign_cc-toolchains-ninja_toolchain"></a>

### `@rules_foreign_cc//toolchains:ninja_toolchain`

Rule `toolchain_type`.
Pin [MODULE.bazel](MODULE.bazel#L66).

<a id="rules_foreign_cc-toolchains-pkgconfig_toolchain"></a>

### `@rules_foreign_cc//toolchains:pkgconfig_toolchain`

Rule `toolchain_type`.
Pin [MODULE.bazel](MODULE.bazel#L66).

<a id="shh_darwin_arm64--shh"></a>

### `@shh_darwin_arm64//:shh`

Pin [MODULE.bazel](MODULE.bazel#L136).

<a id="shh_linux_amd64--shh"></a>

### `@shh_linux_amd64//:shh`

Pin [MODULE.bazel](MODULE.bazel#L134).

<a id="shh_linux_arm64--shh"></a>

### `@shh_linux_arm64//:shh`

Pin [MODULE.bazel](MODULE.bazel#L135).

<a id="toybox_src--all_srcs"></a>

### `@toybox_src//:all_srcs`

Rule `filegroup`.
Pin [tools/extensions.bzl](tools/extensions.bzl#L161).

<a id="zig_darwin_arm64--all"></a>

### `@zig_darwin_arm64//:all`

Rule `filegroup`.
Pin [MODULE.bazel](MODULE.bazel#L153).

<a id="zig_linux_amd64--all"></a>

### `@zig_linux_amd64//:all`

Rule `filegroup`.
Pin [MODULE.bazel](MODULE.bazel#L151).

<a id="zig_linux_arm64--all"></a>

### `@zig_linux_arm64//:all`

Rule `filegroup`.
Pin [MODULE.bazel](MODULE.bazel#L152).

<a id="zig_sdk--all"></a>

### `@zig_sdk//:all`

Rule `alias`.
Pin [MODULE.bazel](MODULE.bazel#L36).

<a id="zig_sdk--zig"></a>

### `@zig_sdk//:zig`

Rule `alias`.
Pin [MODULE.bazel](MODULE.bazel#L36).

<a id="zlib_src--all_srcs"></a>

### `@zlib_src//:all_srcs`

Rule `filegroup`.
Pin [tools/extensions.bzl](tools/extensions.bzl#L170).

<a id="zsh_autosuggestions--all"></a>

### `@zsh_autosuggestions//:all`

Rule `filegroup`.
Pin [tools/extensions.bzl](tools/extensions.bzl#L542).

<a id="zsh_history_substring_search--all"></a>

### `@zsh_history_substring_search//:all`

Rule `filegroup`.
Pin [tools/extensions.bzl](tools/extensions.bzl#L560).

<a id="zsh_src--all_srcs"></a>

### `@zsh_src//:all_srcs`

Rule `filegroup`.
Pin [tools/extensions.bzl](tools/extensions.bzl#L191).

## Layer 1

<a id="macos-exec_toolbox"></a>

### `//macos:exec_toolbox`

Rule `_exec_toolbox`.
File [macos/BUILD.bazel](macos/BUILD.bazel#L13).

Inputs:

- [`//macos:toolbox.miniconfig`](#macos-toolbox.miniconfig)
- [`//macos:toolbox.sh`](#macos-toolbox.sh)
- [`@macos_sdk//:sdk.tar`](#macos_sdk--sdk.tar)
- [`@toybox_src//:all_srcs`](#toybox_src--all_srcs)
- [`@zig_sdk//:all`](#zig_sdk--all)
- [`@zig_sdk//:zig`](#zig_sdk--zig)

<a id="third_party-age-age"></a>

### `//third_party/age:age`

Rule `alias`.
File [third_party/age/BUILD.bazel](third_party/age/BUILD.bazel#L3).

Inputs:

- [`@age_darwin_arm64//:age`](#age_darwin_arm64--age)
- [`@age_linux_amd64//:age`](#age_linux_amd64--age)
- [`@age_linux_arm64//:age`](#age_linux_arm64--age)

<a id="third_party-age-age-keygen"></a>

### `//third_party/age:age-keygen`

Rule `alias`.
File [third_party/age/BUILD.bazel](third_party/age/BUILD.bazel#L13).

Inputs:

- [`@age_darwin_arm64//:age-keygen`](#age_darwin_arm64--age-keygen)
- [`@age_linux_amd64//:age-keygen`](#age_linux_amd64--age-keygen)
- [`@age_linux_arm64//:age-keygen`](#age_linux_arm64--age-keygen)

<a id="third_party-bat-bat"></a>

### `//third_party/bat:bat`

Rule `alias`.
File [third_party/bat/BUILD.bazel](third_party/bat/BUILD.bazel#L2).

Inputs:

- [`@bat_darwin_arm64//:bat`](#bat_darwin_arm64--bat)
- [`@bat_linux_amd64//:bat`](#bat_linux_amd64--bat)
- [`@bat_linux_arm64//:bat`](#bat_linux_arm64--bat)

<a id="third_party-bat_extras-bat_extras"></a>

### `//third_party/bat_extras:bat_extras`

Rule `alias`.
File [third_party/bat_extras/BUILD.bazel](third_party/bat_extras/BUILD.bazel#L3).

Inputs:

- [`@bat_extras//:all`](#bat_extras--all)

<a id="third_party-btop-btop"></a>

### `//third_party/btop:btop`

Rule `cmake`.
File [third_party/btop/BUILD.bazel](third_party/btop/BUILD.bazel#L4).

Inputs:

- [`@bazel_tools//tools/cpp:current_cc_toolchain`](#bazel_tools-tools-cpp-current_cc_toolchain)
- [`@bazel_tools//tools/cpp:toolchain_type`](#bazel_tools-tools-cpp-toolchain_type)
- [`@btop_src//:all_srcs`](#btop_src--all_srcs)
- [`@platforms//os:macos`](#platforms-os-macos)
- [`@rules_foreign_cc//foreign_cc/private/framework:platform_info`](#rules_foreign_cc-foreign_cc-private-framework-platform_info)
- [`@rules_foreign_cc//foreign_cc/private/framework:shell_toolchain`](#rules_foreign_cc-foreign_cc-private-framework-shell_toolchain)
- [`@rules_foreign_cc//toolchains:cmake_toolchain`](#rules_foreign_cc-toolchains-cmake_toolchain)
- [`@rules_foreign_cc//toolchains:m4_toolchain`](#rules_foreign_cc-toolchains-m4_toolchain)
- [`@rules_foreign_cc//toolchains:make_toolchain`](#rules_foreign_cc-toolchains-make_toolchain)
- [`@rules_foreign_cc//toolchains:ninja_toolchain`](#rules_foreign_cc-toolchains-ninja_toolchain)
- [`@rules_foreign_cc//toolchains:pkgconfig_toolchain`](#rules_foreign_cc-toolchains-pkgconfig_toolchain)

<a id="third_party-delta-delta"></a>

### `//third_party/delta:delta`

Rule `alias`.
File [third_party/delta/BUILD.bazel](third_party/delta/BUILD.bazel#L3).

Inputs:

- [`@delta_darwin_arm64//:delta`](#delta_darwin_arm64--delta)
- [`@delta_linux_amd64//:delta`](#delta_linux_amd64--delta)
- [`@delta_linux_arm64//:delta`](#delta_linux_arm64--delta)

<a id="third_party-direnv-direnv"></a>

### `//third_party/direnv:direnv`

Rule `alias`.
File [third_party/direnv/BUILD.bazel](third_party/direnv/BUILD.bazel#L2).

Inputs:

- [`@direnv_darwin_arm64//file:direnv`](#direnv_darwin_arm64-file-direnv)
- [`@direnv_linux_amd64//file:direnv`](#direnv_linux_amd64-file-direnv)
- [`@direnv_linux_arm64//file:direnv`](#direnv_linux_arm64-file-direnv)

<a id="third_party-dust-dust"></a>

### `//third_party/dust:dust`

Rule `alias`.
File [third_party/dust/BUILD.bazel](third_party/dust/BUILD.bazel#L3).

Inputs:

- [`@dust_darwin_arm64//:dust`](#dust_darwin_arm64--dust)
- [`@dust_linux_amd64//:dust`](#dust_linux_amd64--dust)
- [`@dust_linux_arm64//:dust`](#dust_linux_arm64--dust)

<a id="third_party-eza-eza"></a>

### `//third_party/eza:eza`

Rule `genrule`.
File [third_party/eza/BUILD.bazel](third_party/eza/BUILD.bazel#L6).

Inputs:

- [`@bazel_tools//tools/genrule:genrule-setup.sh`](#bazel_tools-tools-genrule-genrule-setup.sh)

<a id="third_party-fd-fd"></a>

### `//third_party/fd:fd`

Rule `alias`.
File [third_party/fd/BUILD.bazel](third_party/fd/BUILD.bazel#L2).

Inputs:

- [`@fd_darwin_arm64//:fd`](#fd_darwin_arm64--fd)
- [`@fd_linux_amd64//:fd`](#fd_linux_amd64--fd)
- [`@fd_linux_arm64//:fd`](#fd_linux_arm64--fd)

<a id="third_party-fzf-fzf"></a>

### `//third_party/fzf:fzf`

Rule `alias`.
File [third_party/fzf/BUILD.bazel](third_party/fzf/BUILD.bazel#L2).

Inputs:

- [`@fzf_darwin_arm64//:fzf`](#fzf_darwin_arm64--fzf)
- [`@fzf_linux_amd64//:fzf`](#fzf_linux_amd64--fzf)
- [`@fzf_linux_arm64//:fzf`](#fzf_linux_arm64--fzf)

<a id="third_party-fzf-shell"></a>

### `//third_party/fzf:shell`

Rule `alias`.
File [third_party/fzf/BUILD.bazel](third_party/fzf/BUILD.bazel#L13).

Inputs:

- [`@fzf_shell//:shell_scripts`](#fzf_shell--shell_scripts)

<a id="third_party-go_sdk-go_sdk"></a>

### `//third_party/go_sdk:go_sdk`

Rule `alias`.
File [third_party/go_sdk/BUILD.bazel](third_party/go_sdk/BUILD.bazel#L2).

Inputs:

- [`@go_sdk_darwin_arm64//:all`](#go_sdk_darwin_arm64--all)
- [`@go_sdk_linux_amd64//:all`](#go_sdk_linux_amd64--all)
- [`@go_sdk_linux_arm64//:all`](#go_sdk_linux_arm64--all)

<a id="third_party-jq-jq"></a>

### `//third_party/jq:jq`

Rule `alias`.
File [third_party/jq/BUILD.bazel](third_party/jq/BUILD.bazel#L2).

Inputs:

- [`@jq_darwin_arm64//file:jq`](#jq_darwin_arm64-file-jq)
- [`@jq_linux_amd64//file:jq`](#jq_linux_amd64-file-jq)
- [`@jq_linux_arm64//file:jq`](#jq_linux_arm64-file-jq)

<a id="third_party-lazygit-lazygit"></a>

### `//third_party/lazygit:lazygit`

Rule `alias`.
File [third_party/lazygit/BUILD.bazel](third_party/lazygit/BUILD.bazel#L2).

Inputs:

- [`@lazygit_darwin_arm64//:lazygit`](#lazygit_darwin_arm64--lazygit)
- [`@lazygit_linux_amd64//:lazygit`](#lazygit_linux_amd64--lazygit)
- [`@lazygit_linux_arm64//:lazygit`](#lazygit_linux_arm64--lazygit)

<a id="third_party-make-gnumake"></a>

### `//third_party/make:gnumake`

Rule `configure_make`.
File [third_party/make/BUILD.bazel](third_party/make/BUILD.bazel#L6).

Inputs:

- [`@bazel_tools//tools/cpp:current_cc_toolchain`](#bazel_tools-tools-cpp-current_cc_toolchain)
- [`@bazel_tools//tools/cpp:toolchain_type`](#bazel_tools-tools-cpp-toolchain_type)
- [`@make_src//:all_srcs`](#make_src--all_srcs)
- [`@platforms//os:macos`](#platforms-os-macos)
- [`@rules_foreign_cc//foreign_cc/private/framework:platform_info`](#rules_foreign_cc-foreign_cc-private-framework-platform_info)
- [`@rules_foreign_cc//foreign_cc/private/framework:shell_toolchain`](#rules_foreign_cc-foreign_cc-private-framework-shell_toolchain)
- [`@rules_foreign_cc//toolchains:autoconf_toolchain`](#rules_foreign_cc-toolchains-autoconf_toolchain)
- [`@rules_foreign_cc//toolchains:automake_toolchain`](#rules_foreign_cc-toolchains-automake_toolchain)
- [`@rules_foreign_cc//toolchains:m4_toolchain`](#rules_foreign_cc-toolchains-m4_toolchain)
- [`@rules_foreign_cc//toolchains:make_toolchain`](#rules_foreign_cc-toolchains-make_toolchain)
- [`@rules_foreign_cc//toolchains:pkgconfig_toolchain`](#rules_foreign_cc-toolchains-pkgconfig_toolchain)

<a id="third_party-ncurses-ncurses"></a>

### `//third_party/ncurses:ncurses`

Rule `configure_make`.
File [third_party/ncurses/BUILD.bazel](third_party/ncurses/BUILD.bazel#L4).

Inputs:

- [`@bazel_tools//tools/cpp:current_cc_toolchain`](#bazel_tools-tools-cpp-current_cc_toolchain)
- [`@bazel_tools//tools/cpp:toolchain_type`](#bazel_tools-tools-cpp-toolchain_type)
- [`@ncurses_src//:all_srcs`](#ncurses_src--all_srcs)
- [`@platforms//os:macos`](#platforms-os-macos)
- [`@rules_foreign_cc//foreign_cc/private/framework:platform_info`](#rules_foreign_cc-foreign_cc-private-framework-platform_info)
- [`@rules_foreign_cc//foreign_cc/private/framework:shell_toolchain`](#rules_foreign_cc-foreign_cc-private-framework-shell_toolchain)
- [`@rules_foreign_cc//toolchains:autoconf_toolchain`](#rules_foreign_cc-toolchains-autoconf_toolchain)
- [`@rules_foreign_cc//toolchains:automake_toolchain`](#rules_foreign_cc-toolchains-automake_toolchain)
- [`@rules_foreign_cc//toolchains:m4_toolchain`](#rules_foreign_cc-toolchains-m4_toolchain)
- [`@rules_foreign_cc//toolchains:make_toolchain`](#rules_foreign_cc-toolchains-make_toolchain)
- [`@rules_foreign_cc//toolchains:pkgconfig_toolchain`](#rules_foreign_cc-toolchains-pkgconfig_toolchain)

<a id="third_party-nvim-nvim"></a>

### `//third_party/nvim:nvim`

Rule `alias`.
File [third_party/nvim/BUILD.bazel](third_party/nvim/BUILD.bazel#L12).

Inputs:

- [`@nvim_darwin_arm64//:all`](#nvim_darwin_arm64--all)
- [`@nvim_linux_amd64//:all`](#nvim_linux_amd64--all)

<a id="third_party-nvim-nvim_deps"></a>

### `//third_party/nvim:nvim_deps`

Rule `cmake`.
File [third_party/nvim/BUILD.bazel](third_party/nvim/BUILD.bazel#L22).

Inputs:

- [`//third_party/nvim:host_cc.sh`](#third_party-nvim-host_cc.sh)
- [`//third_party/nvim:zig_ranlib.sh`](#third_party-nvim-zig_ranlib.sh)
- [`@bazel_tools//tools/cpp:current_cc_toolchain`](#bazel_tools-tools-cpp-current_cc_toolchain)
- [`@bazel_tools//tools/cpp:toolchain_type`](#bazel_tools-tools-cpp-toolchain_type)
- [`@macos_sdk//:sdk.tar`](#macos_sdk--sdk.tar)
- [`@nvim_libuv//file:archive.tar.gz`](#nvim_libuv-file-archive.tar.gz)
- [`@nvim_lpeg//file:archive.tar.gz`](#nvim_lpeg-file-archive.tar.gz)
- [`@nvim_lua_compat53//file:archive.tar.gz`](#nvim_lua_compat53-file-archive.tar.gz)
- [`@nvim_luajit//file:archive.tar.gz`](#nvim_luajit-file-archive.tar.gz)
- [`@nvim_luv//file:archive.tar.gz`](#nvim_luv-file-archive.tar.gz)
- [`@nvim_src//:all_srcs`](#nvim_src--all_srcs)
- [`@nvim_treesitter//file:archive.tar.gz`](#nvim_treesitter-file-archive.tar.gz)
- [`@nvim_treesitter_c//file:archive.tar.gz`](#nvim_treesitter_c-file-archive.tar.gz)
- [`@nvim_treesitter_lua//file:archive.tar.gz`](#nvim_treesitter_lua-file-archive.tar.gz)
- [`@nvim_treesitter_markdown//file:archive.tar.gz`](#nvim_treesitter_markdown-file-archive.tar.gz)
- [`@nvim_treesitter_query//file:archive.tar.gz`](#nvim_treesitter_query-file-archive.tar.gz)
- [`@nvim_treesitter_vim//file:archive.tar.gz`](#nvim_treesitter_vim-file-archive.tar.gz)
- [`@nvim_treesitter_vimdoc//file:archive.tar.gz`](#nvim_treesitter_vimdoc-file-archive.tar.gz)
- [`@nvim_unibilium//file:archive.tar.gz`](#nvim_unibilium-file-archive.tar.gz)
- [`@nvim_utf8proc//file:archive.tar.gz`](#nvim_utf8proc-file-archive.tar.gz)
- [`@platforms//os:macos`](#platforms-os-macos)
- [`@rules_foreign_cc//foreign_cc/private/framework:platform_info`](#rules_foreign_cc-foreign_cc-private-framework-platform_info)
- [`@rules_foreign_cc//foreign_cc/private/framework:shell_toolchain`](#rules_foreign_cc-foreign_cc-private-framework-shell_toolchain)
- [`@rules_foreign_cc//toolchains:cmake_toolchain`](#rules_foreign_cc-toolchains-cmake_toolchain)
- [`@rules_foreign_cc//toolchains:m4_toolchain`](#rules_foreign_cc-toolchains-m4_toolchain)
- [`@rules_foreign_cc//toolchains:make_toolchain`](#rules_foreign_cc-toolchains-make_toolchain)
- [`@rules_foreign_cc//toolchains:ninja_toolchain`](#rules_foreign_cc-toolchains-ninja_toolchain)
- [`@rules_foreign_cc//toolchains:pkgconfig_toolchain`](#rules_foreign_cc-toolchains-pkgconfig_toolchain)
- [`@zig_sdk//:zig`](#zig_sdk--zig)

<a id="third_party-rg-rg"></a>

### `//third_party/rg:rg`

Rule `alias`.
File [third_party/rg/BUILD.bazel](third_party/rg/BUILD.bazel#L3).

Inputs:

- [`@rg_darwin_arm64//:rg`](#rg_darwin_arm64--rg)
- [`@rg_linux_amd64//:rg`](#rg_linux_amd64--rg)
- [`@rg_linux_arm64//:rg`](#rg_linux_arm64--rg)

<a id="third_party-shh-shh"></a>

### `//third_party/shh:shh`

Rule `alias`.
File [third_party/shh/BUILD.bazel](third_party/shh/BUILD.bazel#L1).

Inputs:

- [`@shh_darwin_arm64//:shh`](#shh_darwin_arm64--shh)
- [`@shh_linux_amd64//:shh`](#shh_linux_amd64--shh)
- [`@shh_linux_arm64//:shh`](#shh_linux_arm64--shh)

<a id="third_party-zig-zig"></a>

### `//third_party/zig:zig`

Rule `alias`.
File [third_party/zig/BUILD.bazel](third_party/zig/BUILD.bazel#L2).

Inputs:

- [`@zig_darwin_arm64//:all`](#zig_darwin_arm64--all)
- [`@zig_linux_amd64//:all`](#zig_linux_amd64--all)
- [`@zig_linux_arm64//:all`](#zig_linux_arm64--all)

<a id="third_party-zlib-zlib"></a>

### `//third_party/zlib:zlib`

Rule `cmake`.
File [third_party/zlib/BUILD.bazel](third_party/zlib/BUILD.bazel#L4).

Inputs:

- [`@bazel_tools//tools/cpp:current_cc_toolchain`](#bazel_tools-tools-cpp-current_cc_toolchain)
- [`@bazel_tools//tools/cpp:toolchain_type`](#bazel_tools-tools-cpp-toolchain_type)
- [`@platforms//os:macos`](#platforms-os-macos)
- [`@rules_foreign_cc//foreign_cc/private/framework:platform_info`](#rules_foreign_cc-foreign_cc-private-framework-platform_info)
- [`@rules_foreign_cc//foreign_cc/private/framework:shell_toolchain`](#rules_foreign_cc-foreign_cc-private-framework-shell_toolchain)
- [`@rules_foreign_cc//toolchains:cmake_toolchain`](#rules_foreign_cc-toolchains-cmake_toolchain)
- [`@rules_foreign_cc//toolchains:m4_toolchain`](#rules_foreign_cc-toolchains-m4_toolchain)
- [`@rules_foreign_cc//toolchains:make_toolchain`](#rules_foreign_cc-toolchains-make_toolchain)
- [`@rules_foreign_cc//toolchains:ninja_toolchain`](#rules_foreign_cc-toolchains-ninja_toolchain)
- [`@rules_foreign_cc//toolchains:pkgconfig_toolchain`](#rules_foreign_cc-toolchains-pkgconfig_toolchain)
- [`@zlib_src//:all_srcs`](#zlib_src--all_srcs)

<a id="third_party-zsh_plugins-autosuggestions"></a>

### `//third_party/zsh_plugins:autosuggestions`

Rule `alias`.
File [third_party/zsh_plugins/BUILD.bazel](third_party/zsh_plugins/BUILD.bazel#L3).

Inputs:

- [`@zsh_autosuggestions//:all`](#zsh_autosuggestions--all)

<a id="third_party-zsh_plugins-fast_syntax_highlighting"></a>

### `//third_party/zsh_plugins:fast_syntax_highlighting`

Rule `alias`.
File [third_party/zsh_plugins/BUILD.bazel](third_party/zsh_plugins/BUILD.bazel#L9).

Inputs:

- [`@fast_syntax_highlighting//:all`](#fast_syntax_highlighting--all)

<a id="third_party-zsh_plugins-history_substring_search"></a>

### `//third_party/zsh_plugins:history_substring_search`

Rule `alias`.
File [third_party/zsh_plugins/BUILD.bazel](third_party/zsh_plugins/BUILD.bazel#L15).

Inputs:

- [`@zsh_history_substring_search//:all`](#zsh_history_substring_search--all)

<a id="third_party-zsh_plugins-powerlevel10k"></a>

### `//third_party/zsh_plugins:powerlevel10k`

Rule `alias`.
File [third_party/zsh_plugins/BUILD.bazel](third_party/zsh_plugins/BUILD.bazel#L21).

Inputs:

- [`@powerlevel10k//:all`](#powerlevel10k--all)

## Layer 2

<a id="linux-btop_aarch64"></a>

### `//linux:btop_aarch64`

Rule `flat_archive`.
File [linux/BUILD.bazel](linux/BUILD.bazel#L13).

Inputs:

- [`//third_party/btop:btop`](#third_party-btop-btop)
- [`//tools/packarchive:packarchive`](#tools-packarchive-packarchive)
- [`@bazel_tools//tools/allowlists/function_transition_allowlist:function_transition_allowlist`](#bazel_tools-tools-allowlists-function_transition_allowlist-function_transition_allowlist)

<a id="linux-btop_x86_64"></a>

### `//linux:btop_x86_64`

Rule `flat_archive`.
File [linux/BUILD.bazel](linux/BUILD.bazel#L11).

Inputs:

- [`//third_party/btop:btop`](#third_party-btop-btop)
- [`//tools/packarchive:packarchive`](#tools-packarchive-packarchive)
- [`@bazel_tools//tools/allowlists/function_transition_allowlist:function_transition_allowlist`](#bazel_tools-tools-allowlists-function_transition_allowlist-function_transition_allowlist)

<a id="linux-make_aarch64"></a>

### `//linux:make_aarch64`

Rule `flat_archive`.
File [linux/BUILD.bazel](linux/BUILD.bazel#L13).

Inputs:

- [`//third_party/make:gnumake`](#third_party-make-gnumake)
- [`//tools/packarchive:packarchive`](#tools-packarchive-packarchive)
- [`@bazel_tools//tools/allowlists/function_transition_allowlist:function_transition_allowlist`](#bazel_tools-tools-allowlists-function_transition_allowlist-function_transition_allowlist)

<a id="linux-make_x86_64"></a>

### `//linux:make_x86_64`

Rule `flat_archive`.
File [linux/BUILD.bazel](linux/BUILD.bazel#L11).

Inputs:

- [`//third_party/make:gnumake`](#third_party-make-gnumake)
- [`//tools/packarchive:packarchive`](#tools-packarchive-packarchive)
- [`@bazel_tools//tools/allowlists/function_transition_allowlist:function_transition_allowlist`](#bazel_tools-tools-allowlists-function_transition_allowlist-function_transition_allowlist)

<a id="linux-ncurses_aarch64"></a>

### `//linux:ncurses_aarch64`

Rule `flat_archive`.
File [linux/BUILD.bazel](linux/BUILD.bazel#L13).

Inputs:

- [`//third_party/ncurses:ncurses`](#third_party-ncurses-ncurses)
- [`//tools/packarchive:packarchive`](#tools-packarchive-packarchive)
- [`@bazel_tools//tools/allowlists/function_transition_allowlist:function_transition_allowlist`](#bazel_tools-tools-allowlists-function_transition_allowlist-function_transition_allowlist)

<a id="linux-ncurses_x86_64"></a>

### `//linux:ncurses_x86_64`

Rule `flat_archive`.
File [linux/BUILD.bazel](linux/BUILD.bazel#L11).

Inputs:

- [`//third_party/ncurses:ncurses`](#third_party-ncurses-ncurses)
- [`//tools/packarchive:packarchive`](#tools-packarchive-packarchive)
- [`@bazel_tools//tools/allowlists/function_transition_allowlist:function_transition_allowlist`](#bazel_tools-tools-allowlists-function_transition_allowlist-function_transition_allowlist)

<a id="linux-zlib_aarch64"></a>

### `//linux:zlib_aarch64`

Rule `flat_archive`.
File [linux/BUILD.bazel](linux/BUILD.bazel#L13).

Inputs:

- [`//third_party/zlib:zlib`](#third_party-zlib-zlib)
- [`//tools/packarchive:packarchive`](#tools-packarchive-packarchive)
- [`@bazel_tools//tools/allowlists/function_transition_allowlist:function_transition_allowlist`](#bazel_tools-tools-allowlists-function_transition_allowlist-function_transition_allowlist)

<a id="linux-zlib_x86_64"></a>

### `//linux:zlib_x86_64`

Rule `flat_archive`.
File [linux/BUILD.bazel](linux/BUILD.bazel#L11).

Inputs:

- [`//third_party/zlib:zlib`](#third_party-zlib-zlib)
- [`//tools/packarchive:packarchive`](#tools-packarchive-packarchive)
- [`@bazel_tools//tools/allowlists/function_transition_allowlist:function_transition_allowlist`](#bazel_tools-tools-allowlists-function_transition_allowlist-function_transition_allowlist)

<a id="macos-exec_make"></a>

### `//macos:exec_make`

Rule `_exec_make`.
File [macos/BUILD.bazel](macos/BUILD.bazel#L13).

Inputs:

- [`//macos:exec_toolbox`](#macos-exec_toolbox)
- [`//macos:host_make.sh`](#macos-host_make.sh)
- [`@macos_sdk//:sdk.tar`](#macos_sdk--sdk.tar)
- [`@make_src//:all_srcs`](#make_src--all_srcs)
- [`@zig_sdk//:all`](#zig_sdk--all)
- [`@zig_sdk//:zig`](#zig_sdk--zig)

<a id="macos-libuv"></a>

### `//macos:libuv`

Rule `_macho_lib`.
File [macos/BUILD.bazel](macos/BUILD.bazel#L13).

Inputs:

- [`//macos:archive.sh`](#macos-archive.sh)
- [`//macos:exec_toolbox`](#macos-exec_toolbox)
- [`@macos_sdk//:sdk.tar`](#macos_sdk--sdk.tar)
- [`@nvim_libuv//file:archive.tar.gz`](#nvim_libuv-file-archive.tar.gz)
- [`@zig_sdk//:all`](#zig_sdk--all)
- [`@zig_sdk//:zig`](#zig_sdk--zig)

<a id="macos-lpeg"></a>

### `//macos:lpeg`

Rule `_macho_lib`.
File [macos/BUILD.bazel](macos/BUILD.bazel#L13).

Inputs:

- [`//macos:archive.sh`](#macos-archive.sh)
- [`//macos:exec_toolbox`](#macos-exec_toolbox)
- [`@macos_sdk//:sdk.tar`](#macos_sdk--sdk.tar)
- [`@nvim_lpeg//file:archive.tar.gz`](#nvim_lpeg-file-archive.tar.gz)
- [`@nvim_luajit//file:archive.tar.gz`](#nvim_luajit-file-archive.tar.gz)
- [`@zig_sdk//:all`](#zig_sdk--all)
- [`@zig_sdk//:zig`](#zig_sdk--zig)

<a id="macos-luv"></a>

### `//macos:luv`

Rule `_macho_lib`.
File [macos/BUILD.bazel](macos/BUILD.bazel#L13).

Inputs:

- [`//macos:archive.sh`](#macos-archive.sh)
- [`//macos:exec_toolbox`](#macos-exec_toolbox)
- [`@macos_sdk//:sdk.tar`](#macos_sdk--sdk.tar)
- [`@nvim_libuv//file:archive.tar.gz`](#nvim_libuv-file-archive.tar.gz)
- [`@nvim_lua_compat53//file:archive.tar.gz`](#nvim_lua_compat53-file-archive.tar.gz)
- [`@nvim_luajit//file:archive.tar.gz`](#nvim_luajit-file-archive.tar.gz)
- [`@nvim_luv//file:archive.tar.gz`](#nvim_luv-file-archive.tar.gz)
- [`@zig_sdk//:all`](#zig_sdk--all)
- [`@zig_sdk//:zig`](#zig_sdk--zig)

<a id="macos-tree_sitter"></a>

### `//macos:tree_sitter`

Rule `_macho_lib`.
File [macos/BUILD.bazel](macos/BUILD.bazel#L13).

Inputs:

- [`//macos:archive.sh`](#macos-archive.sh)
- [`//macos:exec_toolbox`](#macos-exec_toolbox)
- [`@macos_sdk//:sdk.tar`](#macos_sdk--sdk.tar)
- [`@nvim_treesitter//file:archive.tar.gz`](#nvim_treesitter-file-archive.tar.gz)
- [`@zig_sdk//:all`](#zig_sdk--all)
- [`@zig_sdk//:zig`](#zig_sdk--zig)

<a id="macos-unibilium"></a>

### `//macos:unibilium`

Rule `_macho_lib`.
File [macos/BUILD.bazel](macos/BUILD.bazel#L13).

Inputs:

- [`//macos:archive.sh`](#macos-archive.sh)
- [`//macos:exec_toolbox`](#macos-exec_toolbox)
- [`@macos_sdk//:sdk.tar`](#macos_sdk--sdk.tar)
- [`@nvim_unibilium//file:archive.tar.gz`](#nvim_unibilium-file-archive.tar.gz)
- [`@zig_sdk//:all`](#zig_sdk--all)
- [`@zig_sdk//:zig`](#zig_sdk--zig)

<a id="macos-utf8proc"></a>

### `//macos:utf8proc`

Rule `_macho_lib`.
File [macos/BUILD.bazel](macos/BUILD.bazel#L13).

Inputs:

- [`//macos:archive.sh`](#macos-archive.sh)
- [`//macos:exec_toolbox`](#macos-exec_toolbox)
- [`@macos_sdk//:sdk.tar`](#macos_sdk--sdk.tar)
- [`@nvim_utf8proc//file:archive.tar.gz`](#nvim_utf8proc-file-archive.tar.gz)
- [`@zig_sdk//:all`](#zig_sdk--all)
- [`@zig_sdk//:zig`](#zig_sdk--zig)

<a id="third_party-git-git"></a>

### `//third_party/git:git`

Rule `configure_make`.
File [third_party/git/BUILD.bazel](third_party/git/BUILD.bazel#L4).

Inputs:

- [`//third_party/zlib:zlib`](#third_party-zlib-zlib)
- [`@bazel_tools//tools/cpp:current_cc_toolchain`](#bazel_tools-tools-cpp-current_cc_toolchain)
- [`@bazel_tools//tools/cpp:toolchain_type`](#bazel_tools-tools-cpp-toolchain_type)
- [`@git_src//:all_srcs`](#git_src--all_srcs)
- [`@platforms//os:macos`](#platforms-os-macos)
- [`@rules_foreign_cc//foreign_cc/private/framework:platform_info`](#rules_foreign_cc-foreign_cc-private-framework-platform_info)
- [`@rules_foreign_cc//foreign_cc/private/framework:shell_toolchain`](#rules_foreign_cc-foreign_cc-private-framework-shell_toolchain)
- [`@rules_foreign_cc//toolchains:autoconf_toolchain`](#rules_foreign_cc-toolchains-autoconf_toolchain)
- [`@rules_foreign_cc//toolchains:automake_toolchain`](#rules_foreign_cc-toolchains-automake_toolchain)
- [`@rules_foreign_cc//toolchains:m4_toolchain`](#rules_foreign_cc-toolchains-m4_toolchain)
- [`@rules_foreign_cc//toolchains:make_toolchain`](#rules_foreign_cc-toolchains-make_toolchain)
- [`@rules_foreign_cc//toolchains:pkgconfig_toolchain`](#rules_foreign_cc-toolchains-pkgconfig_toolchain)

<a id="third_party-htop-htop"></a>

### `//third_party/htop:htop`

Rule `configure_make`.
File [third_party/htop/BUILD.bazel](third_party/htop/BUILD.bazel#L4).

Inputs:

- [`//third_party/ncurses:ncurses`](#third_party-ncurses-ncurses)
- [`@bazel_tools//tools/cpp:current_cc_toolchain`](#bazel_tools-tools-cpp-current_cc_toolchain)
- [`@bazel_tools//tools/cpp:toolchain_type`](#bazel_tools-tools-cpp-toolchain_type)
- [`@htop_src//:all_srcs`](#htop_src--all_srcs)
- [`@platforms//os:macos`](#platforms-os-macos)
- [`@rules_foreign_cc//foreign_cc/private/framework:platform_info`](#rules_foreign_cc-foreign_cc-private-framework-platform_info)
- [`@rules_foreign_cc//foreign_cc/private/framework:shell_toolchain`](#rules_foreign_cc-foreign_cc-private-framework-shell_toolchain)
- [`@rules_foreign_cc//toolchains:autoconf_toolchain`](#rules_foreign_cc-toolchains-autoconf_toolchain)
- [`@rules_foreign_cc//toolchains:automake_toolchain`](#rules_foreign_cc-toolchains-automake_toolchain)
- [`@rules_foreign_cc//toolchains:m4_toolchain`](#rules_foreign_cc-toolchains-m4_toolchain)
- [`@rules_foreign_cc//toolchains:make_toolchain`](#rules_foreign_cc-toolchains-make_toolchain)
- [`@rules_foreign_cc//toolchains:pkgconfig_toolchain`](#rules_foreign_cc-toolchains-pkgconfig_toolchain)

<a id="third_party-zsh-zsh"></a>

### `//third_party/zsh:zsh`

Rule `configure_make`.
File [third_party/zsh/BUILD.bazel](third_party/zsh/BUILD.bazel#L5).

Inputs:

- [`//third_party/ncurses:ncurses`](#third_party-ncurses-ncurses)
- [`@bazel_tools//tools/cpp:current_cc_toolchain`](#bazel_tools-tools-cpp-current_cc_toolchain)
- [`@bazel_tools//tools/cpp:toolchain_type`](#bazel_tools-tools-cpp-toolchain_type)
- [`@platforms//os:macos`](#platforms-os-macos)
- [`@rules_foreign_cc//foreign_cc/private/framework:platform_info`](#rules_foreign_cc-foreign_cc-private-framework-platform_info)
- [`@rules_foreign_cc//foreign_cc/private/framework:shell_toolchain`](#rules_foreign_cc-foreign_cc-private-framework-shell_toolchain)
- [`@rules_foreign_cc//toolchains:autoconf_toolchain`](#rules_foreign_cc-toolchains-autoconf_toolchain)
- [`@rules_foreign_cc//toolchains:automake_toolchain`](#rules_foreign_cc-toolchains-automake_toolchain)
- [`@rules_foreign_cc//toolchains:m4_toolchain`](#rules_foreign_cc-toolchains-m4_toolchain)
- [`@rules_foreign_cc//toolchains:make_toolchain`](#rules_foreign_cc-toolchains-make_toolchain)
- [`@rules_foreign_cc//toolchains:pkgconfig_toolchain`](#rules_foreign_cc-toolchains-pkgconfig_toolchain)
- [`@zsh_src//:all_srcs`](#zsh_src--all_srcs)

<a id="windows-make_aarch64"></a>

### `//windows:make_aarch64`

Rule `flat_archive`.
File [windows/BUILD.bazel](windows/BUILD.bazel#L11).

Inputs:

- [`//third_party/make:gnumake`](#third_party-make-gnumake)
- [`//tools/packarchive:packarchive`](#tools-packarchive-packarchive)
- [`@bazel_tools//tools/allowlists/function_transition_allowlist:function_transition_allowlist`](#bazel_tools-tools-allowlists-function_transition_allowlist-function_transition_allowlist)

<a id="windows-make_x86_64"></a>

### `//windows:make_x86_64`

Rule `flat_archive`.
File [windows/BUILD.bazel](windows/BUILD.bazel#L9).

Inputs:

- [`//third_party/make:gnumake`](#third_party-make-gnumake)
- [`//tools/packarchive:packarchive`](#tools-packarchive-packarchive)
- [`@bazel_tools//tools/allowlists/function_transition_allowlist:function_transition_allowlist`](#bazel_tools-tools-allowlists-function_transition_allowlist-function_transition_allowlist)

## Layer 3

<a id="linux-git_aarch64"></a>

### `//linux:git_aarch64`

Rule `tree_archive`.
File [linux/BUILD.bazel](linux/BUILD.bazel#L13).

Inputs:

- [`//third_party/git:git`](#third_party-git-git)
- [`//tools/packarchive:packarchive`](#tools-packarchive-packarchive)
- [`@bazel_tools//tools/allowlists/function_transition_allowlist:function_transition_allowlist`](#bazel_tools-tools-allowlists-function_transition_allowlist-function_transition_allowlist)

<a id="linux-git_x86_64"></a>

### `//linux:git_x86_64`

Rule `tree_archive`.
File [linux/BUILD.bazel](linux/BUILD.bazel#L11).

Inputs:

- [`//third_party/git:git`](#third_party-git-git)
- [`//tools/packarchive:packarchive`](#tools-packarchive-packarchive)
- [`@bazel_tools//tools/allowlists/function_transition_allowlist:function_transition_allowlist`](#bazel_tools-tools-allowlists-function_transition_allowlist-function_transition_allowlist)

<a id="linux-htop_aarch64"></a>

### `//linux:htop_aarch64`

Rule `flat_archive`.
File [linux/BUILD.bazel](linux/BUILD.bazel#L13).

Inputs:

- [`//third_party/htop:htop`](#third_party-htop-htop)
- [`//tools/packarchive:packarchive`](#tools-packarchive-packarchive)
- [`@bazel_tools//tools/allowlists/function_transition_allowlist:function_transition_allowlist`](#bazel_tools-tools-allowlists-function_transition_allowlist-function_transition_allowlist)

<a id="linux-htop_x86_64"></a>

### `//linux:htop_x86_64`

Rule `flat_archive`.
File [linux/BUILD.bazel](linux/BUILD.bazel#L11).

Inputs:

- [`//third_party/htop:htop`](#third_party-htop-htop)
- [`//tools/packarchive:packarchive`](#tools-packarchive-packarchive)
- [`@bazel_tools//tools/allowlists/function_transition_allowlist:function_transition_allowlist`](#bazel_tools-tools-allowlists-function_transition_allowlist-function_transition_allowlist)

<a id="linux-zsh_aarch64"></a>

### `//linux:zsh_aarch64`

Rule `tree_archive`.
File [linux/BUILD.bazel](linux/BUILD.bazel#L13).

Inputs:

- [`//third_party/zsh:zsh`](#third_party-zsh-zsh)
- [`//tools/packarchive:packarchive`](#tools-packarchive-packarchive)
- [`@bazel_tools//tools/allowlists/function_transition_allowlist:function_transition_allowlist`](#bazel_tools-tools-allowlists-function_transition_allowlist-function_transition_allowlist)

<a id="linux-zsh_x86_64"></a>

### `//linux:zsh_x86_64`

Rule `tree_archive`.
File [linux/BUILD.bazel](linux/BUILD.bazel#L11).

Inputs:

- [`//third_party/zsh:zsh`](#third_party-zsh-zsh)
- [`//tools/packarchive:packarchive`](#tools-packarchive-packarchive)
- [`@bazel_tools//tools/allowlists/function_transition_allowlist:function_transition_allowlist`](#bazel_tools-tools-allowlists-function_transition_allowlist-function_transition_allowlist)

<a id="macos-luajit_aarch64"></a>

### `//macos:luajit_aarch64`

Rule `_macho_luajit`.
File [macos/BUILD.bazel](macos/BUILD.bazel#L13).

Inputs:

- [`//macos:exec_make`](#macos-exec_make)
- [`//macos:exec_toolbox`](#macos-exec_toolbox)
- [`//macos:luajit.sh`](#macos-luajit.sh)
- [`@macos_sdk//:sdk.tar`](#macos_sdk--sdk.tar)
- [`@nvim_luajit//file:archive.tar.gz`](#nvim_luajit-file-archive.tar.gz)
- [`@zig_sdk//:all`](#zig_sdk--all)
- [`@zig_sdk//:zig`](#zig_sdk--zig)

<a id="macos-luajit_x86_64"></a>

### `//macos:luajit_x86_64`

Rule `_macho_luajit`.
File [macos/BUILD.bazel](macos/BUILD.bazel#L13).

Inputs:

- [`//macos:exec_make`](#macos-exec_make)
- [`//macos:exec_toolbox`](#macos-exec_toolbox)
- [`//macos:luajit.sh`](#macos-luajit.sh)
- [`@macos_sdk//:sdk.tar`](#macos_sdk--sdk.tar)
- [`@nvim_luajit//file:archive.tar.gz`](#nvim_luajit-file-archive.tar.gz)
- [`@zig_sdk//:all`](#zig_sdk--all)
- [`@zig_sdk//:zig`](#zig_sdk--zig)

<a id="macos-nvim_deps_aarch64"></a>

### `//macos:nvim_deps_aarch64`

Rule `filegroup`.
File [macos/BUILD.bazel](macos/BUILD.bazel#L13).

Inputs:

- [`//macos:libuv`](#macos-libuv)
- [`//macos:lpeg`](#macos-lpeg)
- [`//macos:luv`](#macos-luv)
- [`//macos:tree_sitter`](#macos-tree_sitter)
- [`//macos:unibilium`](#macos-unibilium)
- [`//macos:utf8proc`](#macos-utf8proc)

<a id="third_party-nvim-host_luajit"></a>

### `//third_party/nvim:host_luajit`

Rule `_host_luajit`.
File [third_party/nvim/BUILD.bazel](third_party/nvim/BUILD.bazel#L22).

Inputs:

- [`//macos:exec_make`](#macos-exec_make)
- [`//macos:exec_toolbox`](#macos-exec_toolbox)
- [`//third_party/nvim:host_cc.sh`](#third_party-nvim-host_cc.sh)
- [`//third_party/nvim:host_luajit.sh`](#third_party-nvim-host_luajit.sh)
- [`@macos_sdk//:sdk.tar`](#macos_sdk--sdk.tar)
- [`@nvim_luajit//file:archive.tar.gz`](#nvim_luajit-file-archive.tar.gz)
- [`@zig_sdk//:all`](#zig_sdk--all)
- [`@zig_sdk//:zig`](#zig_sdk--zig)

## Layer 4

<a id="third_party-nvim-host_nlua0"></a>

### `//third_party/nvim:host_nlua0`

Rule `_host_nlua0`.
File [third_party/nvim/BUILD.bazel](third_party/nvim/BUILD.bazel#L22).

Inputs:

- [`//macos:exec_toolbox`](#macos-exec_toolbox)
- [`//third_party/nvim:host_cc.sh`](#third_party-nvim-host_cc.sh)
- [`//third_party/nvim:host_luajit`](#third_party-nvim-host_luajit)
- [`//third_party/nvim:host_nlua0.sh`](#third_party-nvim-host_nlua0.sh)
- [`@macos_sdk//:sdk.tar`](#macos_sdk--sdk.tar)
- [`@nvim_lpeg//file:archive.tar.gz`](#nvim_lpeg-file-archive.tar.gz)
- [`@nvim_src//:all_srcs`](#nvim_src--all_srcs)
- [`@zig_sdk//:all`](#zig_sdk--all)
- [`@zig_sdk//:zig`](#zig_sdk--zig)

## Layer 5

<a id="third_party-nvim-nvim_musl"></a>

### `//third_party/nvim:nvim_musl`

Rule `cmake`.
File [third_party/nvim/BUILD.bazel](third_party/nvim/BUILD.bazel#L22).

Inputs:

- [`//third_party/nvim:host_luajit`](#third_party-nvim-host_luajit)
- [`//third_party/nvim:host_nlua0`](#third_party-nvim-host_nlua0)
- [`//third_party/nvim:host_nvim.sh`](#third_party-nvim-host_nvim.sh)
- [`//third_party/nvim:nvim_deps`](#third_party-nvim-nvim_deps)
- [`@bazel_tools//tools/cpp:current_cc_toolchain`](#bazel_tools-tools-cpp-current_cc_toolchain)
- [`@bazel_tools//tools/cpp:toolchain_type`](#bazel_tools-tools-cpp-toolchain_type)
- [`@nvim_darwin_arm64//:all`](#nvim_darwin_arm64--all)
- [`@nvim_darwin_arm64//:bin/nvim`](#nvim_darwin_arm64--bin-nvim)
- [`@nvim_linux_amd64//:all`](#nvim_linux_amd64--all)
- [`@nvim_linux_amd64//:bin/nvim`](#nvim_linux_amd64--bin-nvim)
- [`@nvim_linux_arm64//:all`](#nvim_linux_arm64--all)
- [`@nvim_linux_arm64//:bin/nvim`](#nvim_linux_arm64--bin-nvim)
- [`@nvim_src//:all_srcs`](#nvim_src--all_srcs)
- [`@platforms//os:macos`](#platforms-os-macos)
- [`@rules_foreign_cc//foreign_cc/private/framework:platform_info`](#rules_foreign_cc-foreign_cc-private-framework-platform_info)
- [`@rules_foreign_cc//foreign_cc/private/framework:shell_toolchain`](#rules_foreign_cc-foreign_cc-private-framework-shell_toolchain)
- [`@rules_foreign_cc//toolchains:cmake_toolchain`](#rules_foreign_cc-toolchains-cmake_toolchain)
- [`@rules_foreign_cc//toolchains:m4_toolchain`](#rules_foreign_cc-toolchains-m4_toolchain)
- [`@rules_foreign_cc//toolchains:make_toolchain`](#rules_foreign_cc-toolchains-make_toolchain)
- [`@rules_foreign_cc//toolchains:ninja_toolchain`](#rules_foreign_cc-toolchains-ninja_toolchain)
- [`@rules_foreign_cc//toolchains:pkgconfig_toolchain`](#rules_foreign_cc-toolchains-pkgconfig_toolchain)

## Layer 6

<a id="linux-nvim_aarch64"></a>

### `//linux:nvim_aarch64`

Rule `tree_archive`.
File [linux/BUILD.bazel](linux/BUILD.bazel#L13).

Inputs:

- [`//third_party/nvim:nvim_musl`](#third_party-nvim-nvim_musl)
- [`//tools/packarchive:packarchive`](#tools-packarchive-packarchive)
- [`@bazel_tools//tools/allowlists/function_transition_allowlist:function_transition_allowlist`](#bazel_tools-tools-allowlists-function_transition_allowlist-function_transition_allowlist)

<a id="linux-nvim_x86_64"></a>

### `//linux:nvim_x86_64`

Rule `tree_archive`.
File [linux/BUILD.bazel](linux/BUILD.bazel#L11).

Inputs:

- [`//third_party/nvim:nvim_musl`](#third_party-nvim-nvim_musl)
- [`//tools/packarchive:packarchive`](#tools-packarchive-packarchive)
- [`@bazel_tools//tools/allowlists/function_transition_allowlist:function_transition_allowlist`](#bazel_tools-tools-allowlists-function_transition_allowlist-function_transition_allowlist)

## Layer 7

<a id="chain-linux_aarch64"></a>

### `//chain:linux_aarch64`

Rule `filegroup`.
File [chain/BUILD.bazel](chain/BUILD.bazel#L8).

Inputs:

- [`//linux:btop_aarch64`](#linux-btop_aarch64)
- [`//linux:git_aarch64`](#linux-git_aarch64)
- [`//linux:htop_aarch64`](#linux-htop_aarch64)
- [`//linux:make_aarch64`](#linux-make_aarch64)
- [`//linux:ncurses_aarch64`](#linux-ncurses_aarch64)
- [`//linux:nvim_aarch64`](#linux-nvim_aarch64)
- [`//linux:zlib_aarch64`](#linux-zlib_aarch64)
- [`//linux:zsh_aarch64`](#linux-zsh_aarch64)

<a id="chain-linux_x86_64"></a>

### `//chain:linux_x86_64`

Rule `filegroup`.
File [chain/BUILD.bazel](chain/BUILD.bazel#L6).

Inputs:

- [`//linux:btop_x86_64`](#linux-btop_x86_64)
- [`//linux:git_x86_64`](#linux-git_x86_64)
- [`//linux:htop_x86_64`](#linux-htop_x86_64)
- [`//linux:make_x86_64`](#linux-make_x86_64)
- [`//linux:ncurses_x86_64`](#linux-ncurses_x86_64)
- [`//linux:nvim_x86_64`](#linux-nvim_x86_64)
- [`//linux:zlib_x86_64`](#linux-zlib_x86_64)
- [`//linux:zsh_x86_64`](#linux-zsh_x86_64)

<a id="linux-bundle_aarch64"></a>

### `//linux:bundle_aarch64`

Rule `genrule`.
File [linux/BUILD.bazel](linux/BUILD.bazel#L13).

Inputs:

- [`//:versions.env`](#-versions.env)
- [`//linux:btop_aarch64`](#linux-btop_aarch64)
- [`//linux:git_aarch64`](#linux-git_aarch64)
- [`//linux:htop_aarch64`](#linux-htop_aarch64)
- [`//linux:make_aarch64`](#linux-make_aarch64)
- [`//linux:nvim_aarch64`](#linux-nvim_aarch64)
- [`//linux:zsh_aarch64`](#linux-zsh_aarch64)
- [`//tools/assemble:assemble`](#tools-assemble-assemble)
- [`@bazel_tools//tools/genrule:genrule-setup.sh`](#bazel_tools-tools-genrule-genrule-setup.sh)

<a id="linux-bundle_x86_64"></a>

### `//linux:bundle_x86_64`

Rule `genrule`.
File [linux/BUILD.bazel](linux/BUILD.bazel#L11).

Inputs:

- [`//:versions.env`](#-versions.env)
- [`//linux:btop_x86_64`](#linux-btop_x86_64)
- [`//linux:git_x86_64`](#linux-git_x86_64)
- [`//linux:htop_x86_64`](#linux-htop_x86_64)
- [`//linux:make_x86_64`](#linux-make_x86_64)
- [`//linux:nvim_x86_64`](#linux-nvim_x86_64)
- [`//linux:zsh_x86_64`](#linux-zsh_x86_64)
- [`//tools/assemble:assemble`](#tools-assemble-assemble)
- [`@bazel_tools//tools/genrule:genrule-setup.sh`](#bazel_tools-tools-genrule-genrule-setup.sh)

