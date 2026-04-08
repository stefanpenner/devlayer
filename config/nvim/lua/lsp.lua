-- Mason (LSP/tool installer)
require("mason").setup()
require("mason-lspconfig").setup({
  ensure_installed = {
    "lua_ls",
    "gopls",
    "ts_ls",
    "jsonls",
    "yamlls",
    "bashls",
  },
})

-- Lazydev (Neovim Lua API completions)
require("lazydev").setup()

-- Completion
require("blink.cmp").setup({
  keymap = { preset = "default" },
  fuzzy = { implementation = "prefer_rust" },
  appearance = { nerd_font_variant = "mono" },
  sources = {
    default = { "lsp", "path", "snippets", "buffer", "lazydev" },
    providers = {
      lazydev = {
        name = "LazyDev",
        module = "lazydev.integrations.blink",
        score_offset = 100,
      },
    },
  },
  completion = {
    accept = { auto_brackets = { enabled = true } },
    menu = { draw = { treesitter = { "lsp" } } },
    documentation = { auto_show = true, auto_show_delay_ms = 200 },
  },
  signature = { enabled = true },
})

-- LSP server configurations (native vim.lsp.config API)
local capabilities = require("blink.cmp").get_lsp_capabilities()

vim.lsp.config.lua_ls = {
  capabilities = capabilities,
  settings = {
    Lua = {
      workspace = { checkThirdParty = false },
      telemetry = { enable = false },
    },
  },
}

vim.lsp.config.gopls = {
  capabilities = capabilities,
  settings = {
    gopls = {
      gofumpt = true,
      analyses = { unusedparams = true, shadow = true },
      staticcheck = true,
    },
  },
}

vim.lsp.config.ts_ls = { capabilities = capabilities }

vim.lsp.config.jsonls = {
  capabilities = capabilities,
  settings = {
    json = {
      schemas = require("schemastore").json.schemas(),
      validate = { enable = true },
    },
  },
}

vim.lsp.config.yamlls = {
  capabilities = capabilities,
  settings = {
    yaml = {
      schemaStore = { enable = false, url = "" },
      schemas = require("schemastore").yaml.schemas(),
    },
  },
}

vim.lsp.config.bashls = { capabilities = capabilities }

vim.lsp.enable({ "lua_ls", "gopls", "ts_ls", "jsonls", "yamlls", "bashls" })

-- Formatting
require("conform").setup({
  formatters_by_ft = {
    lua = { "stylua" },
    go = { "goimports", "gofumpt" },
    javascript = { "prettierd", "prettier", stop_after_first = true },
    typescript = { "prettierd", "prettier", stop_after_first = true },
    json = { "prettierd", "prettier", stop_after_first = true },
    yaml = { "prettierd", "prettier", stop_after_first = true },
  },
  format_on_save = function(bufnr)
    if vim.g.autoformat == false or vim.b[bufnr].autoformat == false then
      return
    end
    return { timeout_ms = 3000, lsp_format = "fallback" }
  end,
})

-- Linting
require("lint").linters_by_ft = {
  go = { "golangcilint" },
}

vim.api.nvim_create_autocmd({ "BufWritePost", "BufReadPost", "InsertLeave" }, {
  callback = function()
    require("lint").try_lint()
  end,
})
