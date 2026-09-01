.PHONY: build test fmt lint clean

build:
	dotnet build Caro.sln -c Release

test:
	dotnet test Caro.sln

fmt:
	dotnet format Caro.sln

lint:
	dotnet format Caro.sln --verify-no-changes

clean:
	dotnet clean Caro.sln
	rm -rf src/*/bin src/*/obj tests/*/bin tests/*/obj
