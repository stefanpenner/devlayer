-- Run TSUpdate after plugins are installed/updated
vim.api.nvim_create_autocmd("User", {
  pattern = "PackChanged",
  callback = function()
    if vim.fn.exists(":TSUpdate") == 2 then
      vim.cmd("TSUpdate")
    end
  end,
})

-- vim.pack requires nvim 0.12+; on older versions, plugins are loaded via packadd below
if vim.pack then
vim.pack.add({
  -- LSP & Completion
  "https://github.com/neovim/nvim-lspconfig",
  "https://github.com/mason-org/mason.nvim",
  "https://github.com/mason-org/mason-lspconfig.nvim",
  "https://github.com/saghen/blink.cmp",
  "https://github.com/folke/lazydev.nvim",
  "https://github.com/rafamadriz/friendly-snippets",

  -- Syntax & Navigation
  "https://github.com/nvim-treesitter/nvim-treesitter",
  "https://github.com/nvim-treesitter/nvim-treesitter-textobjects",
  "https://github.com/windwp/nvim-ts-autotag",
  "https://github.com/folke/flash.nvim",
  "https://github.com/MagicDuck/grug-far.nvim",

  -- Editor
  "https://github.com/lewis6991/gitsigns.nvim",
  "https://github.com/folke/trouble.nvim",
  "https://github.com/folke/todo-comments.nvim",
  "https://github.com/folke/which-key.nvim",
  "https://github.com/echasnovski/mini.ai",
  "https://github.com/echasnovski/mini.pairs",
  "https://github.com/folke/ts-comments.nvim",
  "https://github.com/mg979/vim-visual-multi",
  "https://github.com/folke/persistence.nvim",
  "https://github.com/gbprod/yanky.nvim",

  -- Code Quality
  "https://github.com/stevearc/conform.nvim",
  "https://github.com/mfussenegger/nvim-lint",

  -- UI
  "https://github.com/nvim-lualine/lualine.nvim",
  "https://github.com/akinsho/bufferline.nvim",
  "https://github.com/echasnovski/mini.icons",
  "https://github.com/folke/noice.nvim",
  "https://github.com/MunifTanjim/nui.nvim",
  "https://github.com/folke/snacks.nvim",
  "https://github.com/EdenEast/nightfox.nvim",

  -- Testing
  "https://github.com/nvim-neotest/neotest",
  "https://github.com/fredrikaverpil/neotest-golang",
  "https://github.com/nvim-neotest/nvim-nio",

  -- Rendering
  "https://github.com/MeanderingProgrammer/render-markdown.nvim",
  "https://github.com/iamcco/markdown-preview.nvim",

  -- Libraries
  "https://github.com/nvim-lua/plenary.nvim",
  "https://github.com/b0o/SchemaStore.nvim",
})
end -- vim.pack

-- Load all installed opt packages onto runtimepath
local pack_dir = vim.fn.stdpath("data") .. "/site/pack/core/opt"
for _, path in ipairs(vim.fn.globpath(pack_dir, "*", false, true)) do
  local name = vim.fn.fnamemodify(path, ":t")
  vim.cmd.packadd(name)
end
