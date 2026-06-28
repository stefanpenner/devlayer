local map = vim.keymap.set

-- Better up/down (respect wrapped lines when no count)
map({ "n", "x" }, "j", "v:count == 0 ? 'gj' : 'j'", { desc = "Down", expr = true, silent = true })
map({ "n", "x" }, "<Down>", "v:count == 0 ? 'gj' : 'j'", { desc = "Down", expr = true, silent = true })
map({ "n", "x" }, "k", "v:count == 0 ? 'gk' : 'k'", { desc = "Up", expr = true, silent = true })
map({ "n", "x" }, "<Up>", "v:count == 0 ? 'gk' : 'k'", { desc = "Up", expr = true, silent = true })

-- Better window navigation
map("n", "<C-h>", "<C-w>h", { desc = "Go to left window" })
map("n", "<C-j>", "<C-w>j", { desc = "Go to lower window" })
map("n", "<C-k>", "<C-w>k", { desc = "Go to upper window" })
map("n", "<C-l>", "<C-w>l", { desc = "Go to right window" })

-- Resize windows
map("n", "<C-Up>", "<cmd>resize +2<cr>", { desc = "Increase window height" })
map("n", "<C-Down>", "<cmd>resize -2<cr>", { desc = "Decrease window height" })
map("n", "<C-Left>", "<cmd>vertical resize -2<cr>", { desc = "Decrease window width" })
map("n", "<C-Right>", "<cmd>vertical resize +2<cr>", { desc = "Increase window width" })

-- Buffers
map("n", "<S-h>", "<cmd>bprevious<cr>", { desc = "Prev buffer" })
map("n", "<S-l>", "<cmd>bnext<cr>", { desc = "Next buffer" })
map("n", "[b", "<cmd>bprevious<cr>", { desc = "Prev buffer" })
map("n", "]b", "<cmd>bnext<cr>", { desc = "Next buffer" })
map("n", "<leader>bb", "<cmd>e #<cr>", { desc = "Switch to other buffer" })
map("n", "<leader>`", "<cmd>e #<cr>", { desc = "Switch to other buffer" })
map("n", "<leader>bd", "<cmd>bdelete<cr>", { desc = "Delete buffer" })
map("n", "<leader>bD", "<cmd>bdelete|close<cr>", { desc = "Delete buffer and window" })
map("n", "<leader>bo", "<cmd>%bdelete|edit#|bdelete#<cr>", { desc = "Delete other buffers" })

-- Bufferline
map("n", "<leader>bp", "<cmd>BufferLineTogglePin<cr>", { desc = "Toggle pin" })
map("n", "<leader>bP", "<cmd>BufferLineGroupClose ungrouped<cr>", { desc = "Delete non-pinned buffers" })
map("n", "<leader>br", "<cmd>BufferLineCloseRight<cr>", { desc = "Delete buffers to the right" })
map("n", "<leader>bl", "<cmd>BufferLineCloseLeft<cr>", { desc = "Delete buffers to the left" })
map("n", "[B", "<cmd>BufferLineMovePrev<cr>", { desc = "Move buffer left" })
map("n", "]B", "<cmd>BufferLineMoveNext<cr>", { desc = "Move buffer right" })

-- Move lines
map("n", "<A-j>", "<cmd>m .+1<cr>==", { desc = "Move down" })
map("n", "<A-k>", "<cmd>m .-2<cr>==", { desc = "Move up" })
map("i", "<A-j>", "<esc><cmd>m .+1<cr>==gi", { desc = "Move down" })
map("i", "<A-k>", "<esc><cmd>m .-2<cr>==gi", { desc = "Move up" })
map("v", "<A-j>", ":m '>+1<cr>gv=gv", { desc = "Move down" })
map("v", "<A-k>", ":m '<-2<cr>gv=gv", { desc = "Move up" })

-- Clear search highlight
map({ "i", "n", "s" }, "<esc>", "<cmd>noh<cr><esc>", { desc = "Escape and clear hlsearch" })

-- Search: center and open folds
map({ "n", "x", "o" }, "n", "'Nn'[v:searchforward].'zzzv'", { desc = "Next search result", expr = true })
map({ "n", "x", "o" }, "N", "'nN'[v:searchforward].'zzzv'", { desc = "Prev search result", expr = true })

-- Save file
map({ "i", "x", "n", "s" }, "<C-s>", "<cmd>w<cr><esc>", { desc = "Save file" })

-- Better indenting (stay in visual mode)
map("v", "<", "<gv")
map("v", ">", ">gv")

-- Undo break-points
map("i", ",", ",<c-g>u")
map("i", ".", ".<c-g>u")
map("i", ";", ";<c-g>u")

-- Comment on new line
map("n", "gco", "o<esc>Vcx<esc><cmd>normal gcc<cr>fxa<bs>", { desc = "Add comment below" })
map("n", "gcO", "O<esc>Vcx<esc><cmd>normal gcc<cr>fxa<bs>", { desc = "Add comment above" })

-- Keywordprg
map("n", "<leader>K", "<cmd>norm! K<cr>", { desc = "Keywordprg" })

-- New file
map("n", "<leader>fn", "<cmd>enew<cr>", { desc = "New file" })

-- Windows
map("n", "<leader>ww", "<C-w>p", { desc = "Other window" })
map("n", "<leader>wd", "<C-w>c", { desc = "Delete window" })
map("n", "<leader>w-", "<C-w>s", { desc = "Split window below" })
map("n", "<leader>w|", "<C-w>v", { desc = "Split window right" })
map("n", "<leader>-", "<C-w>s", { desc = "Split window below" })
map("n", "<leader>|", "<C-w>v", { desc = "Split window right" })

-- Tabs
map("n", "<leader><tab>l", "<cmd>tablast<cr>", { desc = "Last tab" })
map("n", "<leader><tab>o", "<cmd>tabonly<cr>", { desc = "Close other tabs" })
map("n", "<leader><tab>f", "<cmd>tabfirst<cr>", { desc = "First tab" })
map("n", "<leader><tab><tab>", "<cmd>tabnew<cr>", { desc = "New tab" })
map("n", "<leader><tab>]", "<cmd>tabnext<cr>", { desc = "Next tab" })
map("n", "<leader><tab>d", "<cmd>tabclose<cr>", { desc = "Close tab" })
map("n", "<leader><tab>[", "<cmd>tabprevious<cr>", { desc = "Previous tab" })

-- Quickfix / location list
map("n", "[q", vim.cmd.cprev, { desc = "Prev quickfix" })
map("n", "]q", vim.cmd.cnext, { desc = "Next quickfix" })

-- Redraw / clear
map("n", "<leader>ur", "<cmd>nohlsearch|diffupdate|normal! <C-L><cr>", { desc = "Redraw / clear hlsearch / diff update" })

-- Terminal
map("n", "<leader>ft", function() Snacks.terminal() end, { desc = "Terminal (root dir)" })
map("n", "<leader>fT", function() Snacks.terminal(nil, { cwd = vim.uv.cwd() }) end, { desc = "Terminal (cwd)" })
map("n", "<C-/>", function() Snacks.terminal() end, { desc = "Terminal" })
map("t", "<C-/>", "<cmd>close<cr>", { desc = "Hide terminal" })

-- Snacks find/search (replaces telescope)
map("n", "<leader>ff", function() Snacks.picker.files() end, { desc = "Find files" })
map("n", "<leader>fg", function() Snacks.picker.grep() end, { desc = "Live grep" })
map("n", "<leader>fb", function() Snacks.picker.buffers() end, { desc = "Find buffers" })
map("n", "<leader>fh", function() Snacks.picker.help() end, { desc = "Help pages" })
map("n", "<leader>fr", function() Snacks.picker.recent() end, { desc = "Recent files" })
map("n", "<leader>fw", function() Snacks.picker.grep_word() end, { desc = "Grep word" })
map("n", "<leader>fc", function() Snacks.picker.files({ cwd = vim.fn.stdpath("config") }) end, { desc = "Find config file" })
map("n", "<leader><space>", function() Snacks.picker.files() end, { desc = "Find files" })
map("n", "<leader>/", function() Snacks.picker.grep() end, { desc = "Grep" })
map("n", "<leader>,", function() Snacks.picker.buffers() end, { desc = "Switch buffer" })
map("n", "<leader>:", function() Snacks.picker.command_history() end, { desc = "Command history" })

-- Search
map("n", "<leader>sd", function() Snacks.picker.diagnostics() end, { desc = "Diagnostics" })
map("n", "<leader>sg", function() Snacks.picker.grep() end, { desc = "Grep" })
map("n", "<leader>sk", function() Snacks.picker.keymaps() end, { desc = "Keymaps" })
map("n", "<leader>ss", function() Snacks.picker.lsp_symbols() end, { desc = "LSP symbols" })
map("n", "<leader>sS", function() Snacks.picker.lsp_workspace_symbols() end, { desc = "LSP workspace symbols" })
map("n", "<leader>sh", function() Snacks.picker.help() end, { desc = "Help pages" })
map("n", "<leader>sm", function() Snacks.picker.marks() end, { desc = "Marks" })
map("n", "<leader>sR", function() Snacks.picker.resume() end, { desc = "Resume" })
map("n", '<leader>s"', function() Snacks.picker.registers() end, { desc = "Registers" })
map("n", "<leader>sC", function() Snacks.picker.commands() end, { desc = "Commands" })

-- File explorer
map("n", "<leader>e", function() Snacks.explorer() end, { desc = "File explorer" })
map("n", "<leader>fe", function() Snacks.explorer() end, { desc = "File explorer" })

-- Git (lazygit themed via ~/.config/lazygit/config.yml)
local lg_cmd = "lazygit"
map("n", "<leader>gg", function() Snacks.terminal(lg_cmd) end, { desc = "Lazygit" })
map("n", "<leader>gb", function() Snacks.picker.git_branches() end, { desc = "Git branches" })
map("n", "<leader>gl", function() Snacks.picker.git_log() end, { desc = "Git log" })
map("n", "<leader>gL", function() Snacks.picker.git_log_line() end, { desc = "Git log (line)" })
map("n", "<leader>gs", function() Snacks.picker.git_status() end, { desc = "Git status" })
map("n", "<leader>gB", function() Snacks.git.blame_line() end, { desc = "Git blame line" })

-- Snacks extras
map("n", "<leader>.", function() Snacks.scratch() end, { desc = "Toggle scratch buffer" })
map("n", "<leader>n", function() Snacks.notifier.show_history() end, { desc = "Notification history" })

-- LSP (via LspAttach autocmd)
vim.api.nvim_create_autocmd("LspAttach", {
  callback = function(ev)
    local buf = ev.buf
    local opts = function(desc) return { buffer = buf, desc = desc } end

    map("n", "gd", vim.lsp.buf.definition, opts("Go to definition"))
    map("n", "gD", vim.lsp.buf.declaration, opts("Go to declaration"))
    map("n", "gr", vim.lsp.buf.references, opts("References"))
    map("n", "gI", vim.lsp.buf.implementation, opts("Go to implementation"))
    map("n", "gy", vim.lsp.buf.type_definition, opts("Go to type definition"))
    map("n", "K", vim.lsp.buf.hover, opts("Hover"))
    map("n", "gK", vim.lsp.buf.signature_help, opts("Signature help"))
    map("i", "<C-k>", vim.lsp.buf.signature_help, opts("Signature help"))
    map("n", "<leader>ca", vim.lsp.buf.code_action, opts("Code action"))
    map("n", "<leader>cr", vim.lsp.buf.rename, opts("Rename"))
    map("n", "<leader>cA", function()
      vim.lsp.buf.code_action({ context = { only = { "source" }, diagnostics = {} } })
    end, opts("Source action"))
    map("n", "<leader>cR", function() Snacks.rename.rename_file() end, opts("Rename file"))
    map("n", "<leader>co", function() vim.lsp.buf.code_action({ context = { only = { "source.organizeImports" }, diagnostics = {} } }) end, opts("Organize imports"))
    map("n", "<leader>cc", vim.lsp.codelens.run, opts("Run code lens"))
    map("n", "<leader>cC", vim.lsp.codelens.refresh, opts("Refresh code lens"))
    map("n", "<leader>cl", "<cmd>checkhealth lsp<cr>", opts("LSP info"))
    map("n", "<leader>cm", "<cmd>Mason<cr>", opts("Mason"))
    map("n", "[[", function() Snacks.words.jump(-1, true) end, opts("Prev reference"))
    map("n", "]]", function() Snacks.words.jump(1, true) end, opts("Next reference"))
    map("n", "<a-n>", function() Snacks.words.jump(1, true) end, opts("Next reference"))
    map("n", "<a-p>", function() Snacks.words.jump(-1, true) end, opts("Prev reference"))
  end,
})

-- Diagnostics
map("n", "]d", vim.diagnostic.goto_next, { desc = "Next diagnostic" })
map("n", "[d", vim.diagnostic.goto_prev, { desc = "Prev diagnostic" })
map("n", "]e", function() vim.diagnostic.goto_next({ severity = vim.diagnostic.severity.ERROR }) end, { desc = "Next error" })
map("n", "[e", function() vim.diagnostic.goto_prev({ severity = vim.diagnostic.severity.ERROR }) end, { desc = "Prev error" })
map("n", "]w", function() vim.diagnostic.goto_next({ severity = vim.diagnostic.severity.WARN }) end, { desc = "Next warning" })
map("n", "[w", function() vim.diagnostic.goto_prev({ severity = vim.diagnostic.severity.WARN }) end, { desc = "Prev warning" })
map("n", "<leader>cd", vim.diagnostic.open_float, { desc = "Line diagnostics" })
map("n", "<leader>xx", "<cmd>Trouble diagnostics toggle<cr>", { desc = "Diagnostics (Trouble)" })
map("n", "<leader>xX", "<cmd>Trouble diagnostics toggle filter.buf=0<cr>", { desc = "Buffer diagnostics" })
map("n", "<leader>xL", "<cmd>Trouble loclist toggle<cr>", { desc = "Location list" })
map("n", "<leader>xQ", "<cmd>Trouble qflist toggle<cr>", { desc = "Quickfix list" })

-- Flash
map({ "n", "x", "o" }, "s", function() require("flash").jump() end, { desc = "Flash" })
map({ "n", "x", "o" }, "S", function() require("flash").treesitter() end, { desc = "Flash treesitter" })
map("o", "r", function() require("flash").remote() end, { desc = "Remote flash" })
map({ "o", "x" }, "R", function() require("flash").treesitter_search() end, { desc = "Treesitter search" })
map("c", "<C-s>", function() require("flash").toggle() end, { desc = "Toggle flash search" })

-- Grug-far (search & replace)
map("n", "<leader>sr", function()
  require("grug-far").open({ prefills = { search = vim.fn.expand("<cword>") } })
end, { desc = "Search and replace" })

-- Todo comments
map("n", "]t", function() require("todo-comments").jump_next() end, { desc = "Next todo" })
map("n", "[t", function() require("todo-comments").jump_prev() end, { desc = "Prev todo" })
map("n", "<leader>xt", "<cmd>Trouble todo toggle<cr>", { desc = "Todo (Trouble)" })
map("n", "<leader>xT", "<cmd>Trouble todo toggle filter = {tag = {TODO,FIX,FIXME}}<cr>", { desc = "Todo/Fix/Fixme" })
map("n", "<leader>st", "<cmd>Trouble todo toggle<cr>", { desc = "Todo" })

-- Testing
map("n", "<leader>tt", function() require("neotest").run.run() end, { desc = "Run nearest test" })
map("n", "<leader>tf", function() require("neotest").run.run(vim.fn.expand("%")) end, { desc = "Run file tests" })
map("n", "<leader>ts", function() require("neotest").summary.toggle() end, { desc = "Test summary" })
map("n", "<leader>to", function() require("neotest").output_panel.toggle() end, { desc = "Test output" })
map("n", "<leader>tS", function() require("neotest").run.stop() end, { desc = "Stop test" })
map("n", "<leader>tl", function() require("neotest").run.run_last() end, { desc = "Run last test" })

-- Yanky
map({ "n", "x" }, "p", "<Plug>(YankyPutAfter)", { desc = "Put after" })
map({ "n", "x" }, "P", "<Plug>(YankyPutBefore)", { desc = "Put before" })
map("n", "<C-p>", "<Plug>(YankyPreviousEntry)", { desc = "Prev yank entry" })
map("n", "<C-n>", "<Plug>(YankyNextEntry)", { desc = "Next yank entry" })

-- Persistence (sessions)
map("n", "<leader>qs", function() require("persistence").load() end, { desc = "Restore session" })
map("n", "<leader>ql", function() require("persistence").load({ last = true }) end, { desc = "Restore last session" })
map("n", "<leader>qd", function() require("persistence").stop() end, { desc = "Don't save session" })

-- Format
map({ "n", "v" }, "<leader>cf", function()
  require("conform").format({ async = true, lsp_format = "fallback" })
end, { desc = "Format" })

-- Noice
map("n", "<leader>snl", function() require("noice").cmd("last") end, { desc = "Noice last message" })
map("n", "<leader>snh", function() require("noice").cmd("history") end, { desc = "Noice history" })
map("n", "<leader>sna", function() require("noice").cmd("all") end, { desc = "Noice all" })
map("n", "<leader>snd", function() require("noice").cmd("dismiss") end, { desc = "Dismiss all" })
map("n", "<leader>snt", function() require("noice").cmd("pick") end, { desc = "Noice picker" })
map({"i", "s"}, "<c-f>", function() if not require("noice.lsp").scroll(4) then return "<c-f>" end end, { silent = true, expr = true, desc = "Scroll forward" })
map({"i", "s"}, "<c-b>", function() if not require("noice.lsp").scroll(-4) then return "<c-b>" end end, { silent = true, expr = true, desc = "Scroll backward" })

-- UI toggles
map("n", "<leader>ud", function() Snacks.toggle.diagnostics() end, { desc = "Toggle diagnostics" })
map("n", "<leader>uf", function()
  vim.g.autoformat = vim.g.autoformat == false
  vim.notify("Format on save: " .. (vim.g.autoformat and "on" or "off"))
end, { desc = "Toggle format on save" })
map("n", "<leader>uw", function() vim.wo.wrap = not vim.wo.wrap end, { desc = "Toggle word wrap" })
map("n", "<leader>ul", function() vim.wo.number = not vim.wo.number end, { desc = "Toggle line numbers" })
map("n", "<leader>uL", function() vim.wo.relativenumber = not vim.wo.relativenumber end, { desc = "Toggle relative numbers" })
map("n", "<leader>us", function() vim.o.spell = not vim.o.spell end, { desc = "Toggle spelling" })
map("n", "<leader>uc", function() local c = vim.o.conceallevel == 0 and 2 or 0; vim.o.conceallevel = c end, { desc = "Toggle conceal" })
map("n", "<leader>uT", function() if vim.b.ts_highlight then vim.treesitter.stop() else vim.treesitter.start() end end, { desc = "Toggle treesitter highlight" })
map("n", "<leader>uh", function() vim.lsp.inlay_hint.enable(not vim.lsp.inlay_hint.is_enabled()) end, { desc = "Toggle inlay hints" })
map("n", "<leader>ug", function() Snacks.toggle.indent() end, { desc = "Toggle indent guides" })
map("n", "<leader>un", function() Snacks.notifier.show_history() end, { desc = "Notification history" })
map("n", "<leader>ui", vim.show_pos, { desc = "Inspect pos" })
map("n", "<leader>uI", "<cmd>InspectTree<cr>", { desc = "Inspect tree" })

-- Quit
map("n", "<leader>qq", "<cmd>qa<cr>", { desc = "Quit all" })
