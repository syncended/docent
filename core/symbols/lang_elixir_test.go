package symbols

import "testing"

func TestExtractElixir(t *testing.T) {
	src := `defmodule MyApp.Accounts do
  @moduledoc "User accounts."
  alias MyApp.Repo
  import Ecto.Query

  defstruct [:name, :email]

  def list_users(opts \\ []), do: Repo.all(opts)

  def get_user!(id) when is_integer(id) do
    loader = fn key -> Repo.get!(User, key) end
    loader.(id)
  end

  defp normalize(email) do
    String.downcase(email)
  end

  defmacro __using__(_opts) do
    quote do
      def injected, do: :ok
    end
  end

  defguard is_admin(user) when user.role == :admin

  defdelegate fetch(id), to: Repo, as: :get

  defmodule Session do
    def current, do: nil
  end
end

defprotocol MyApp.Size do
  def size(data)
end

defimpl MyApp.Size, for: BitString do
  def size(string), do: byte_size(string)
end
`
	want := []string{
		"MyApp.Accounts class",
		"MyApp.Accounts.list_users method",
		"MyApp.Accounts.get_user! method",
		"MyApp.Accounts.normalize method",
		"MyApp.Accounts.__using__ method",
		"MyApp.Accounts.is_admin method",
		"MyApp.Accounts.fetch method",
		"MyApp.Accounts.Session class",
		"Session.current method",
		"MyApp.Size interface",
		"MyApp.Size.size method",
		"size function",
	}
	not := []string{
		"MyApp.Repo", "Ecto.Query", "moduledoc", "defstruct", "loader", "key",
		"injected", "String", "BitString", "is_integer", "byte_size", "all",
	}
	checkExtract(t, "elixir", src, want, not)
}
