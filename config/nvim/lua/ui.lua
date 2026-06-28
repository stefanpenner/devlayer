-- Colorscheme
require("nightfox").setup({
  options = {
    styles = {
      comments = "NONE",
      keywords = "NONE",
      types = "NONE",
    },
  },
})
vim.cmd.colorscheme("carbonfox")

-- Icons (must be set up before plugins that use them)
require("mini.icons").setup()

-- Statusline
require("lualine").setup({
  options = {
    theme = "carbonfox",
    globalstatus = true,
    component_separators = { left = "", right = "" },
    section_separators = { left = "", right = "" },
  },
  sections = {
    lualine_a = { "mode" },
    lualine_b = { "branch", "diff", "diagnostics" },
    lualine_c = { { "filename", path = 1 } },
    lualine_x = { "encoding", "fileformat", "filetype" },
    lualine_y = { "progress" },
    lualine_z = { "location" },
  },
})

-- Buffer tabs
require("bufferline").setup({
  options = {
    diagnostics = "nvim_lsp",
    always_show_bufferline = false,
    offsets = {
      { filetype = "snacks_layout_box", text = "", padding = 1 },
    },
  },
})

-- Noice (UI for messages, cmdline, popups)
require("noice").setup({
  lsp = {
    override = {
      ["vim.lsp.util.convert_input_to_markdown_lines"] = true,
      ["vim.lsp.util.stylize_markdown"] = true,
      ["cmp.entry.get_documentation"] = true,
    },
  },
  presets = {
    bottom_search = true,
    command_palette = true,
    long_message_to_split = true,
  },
})

-- Snacks (dashboard, notifications, indent guides)
require("snacks").setup({
  dashboard = { enabled = false },
  notifier = { enabled = true },
  indent = { enabled = true },
  scroll = { enabled = true },
  statuscolumn = { enabled = true },
  words = { enabled = true },
  explorer = { enabled = true },
})
