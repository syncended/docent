package symbols

import "testing"

func TestExtractLua(t *testing.T) {
	src := `
local M = {
  version = 1,
  format = function(value) return tostring(value) end,
}

function setup(opts)
  local function merge(a, b)
    return a
  end
  local apply = function() end
  return merge(opts, {})
end

function M.open(path)
  return path
end

function M.util.trim(s)
  return s
end

function M:close()
  local helpers = { reset = function() end }
  return helpers
end

local function validate(x)
  return x ~= nil
end

local parse = function(text) return text end
M.render = function() end

return M
`
	checkExtract(t, "lua", src, []string{
		"format function",
		"setup function",
		"open function",
		"trim function",
		"close function",
		"validate function",
		"parse function",
		"render function",
	}, []string{"M", "version", "merge", "apply", "helpers", "reset"})
}
