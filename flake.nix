# Every tool CI, the job image and local dev use. nixpkgs (flake.lock) is the pin:
# bump it and all three move together. Go is nixpkgs' patch, so go.mod keeps
# major.minor. PostgreSQL is not here: the job image installs it and CI uses a
# service container. `nix develop`, or `make flake/build`.
{
  description = "The repo's toolchain: one pin source for CI, the job image and local dev";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";

  outputs =
    { nixpkgs, ... }:
    let
      systems = [
        "aarch64-darwin"
        "aarch64-linux"
        "x86_64-linux"
      ];
      eachSystem = nixpkgs.lib.genAttrs systems;

      # A tool nixpkgs does not carry, or carries at another major version.
      buildGoTool =
        pkgs:
        {
          pname,
          version,
          owner,
          repo,
          srcHash,
          vendorHash,
          subPackages ? [ "." ],
          description,
        }:
        pkgs.buildGoModule {
          inherit pname version vendorHash subPackages;
          src = pkgs.fetchFromGitHub {
            inherit owner repo;
            tag = "v${version}";
            hash = srcHash;
          };
          meta.description = description;
        };

      tools =
        pkgs:
        let
          build = buildGoTool pkgs;
        in
        [
          pkgs.go
          pkgs.nodejs_26
          pkgs.python3
          pkgs.sqlfluff
          pkgs.buf
          pkgs.protoc-gen-go
          (build {
            pname = "protoc-gen-connect-go";
            version = "2.0.2";
            owner = "connectrpc";
            repo = "connect-go";
            srcHash = "sha256-iZfWjbMOE2bWo6IbhwzfSYJCtm5yit/wMLKCY05gs4Q=";
            vendorHash = "sha256-0XmH9V7Bbuzj//87Ev6KRy6ijsgh7K6JRhhM5WY7K38=";
            subPackages = [ "cmd/protoc-gen-connect-go" ];
            description = "Protobuf generator for Connect RPC";
          })
          pkgs.golangci-lint
          pkgs.gci
          pkgs.gosec
          pkgs.golines
          pkgs.govulncheck
          (build {
            pname = "gremlins";
            version = "0.6.0";
            owner = "go-gremlins";
            repo = "gremlins";
            srcHash = "sha256-QwMj7aA4eafMT25gBLAomZMliCbueoEsDHD/nxtnmk4=";
            vendorHash = "sha256-TYbbDN2V6GLj+YRNQIKggCnNspk3M96cP1DSe8P9qlY=";
            subPackages = [ "cmd/gremlins" ];
            description = "Mutation testing for Go";
          })
          (build {
            pname = "godepgraph";
            version = "1.0.0";
            owner = "kisielk";
            repo = "godepgraph";
            srcHash = "sha256-fOI/WGGSKKkhevbbEyaDV9pix/DNMO2Oe+5fEroDK5M=";
            vendorHash = null;
            description = "Go package dependency graph";
          })
          pkgs.git
          pkgs.gnumake
          pkgs.jq
          pkgs.curl
          pkgs.bash
          pkgs.coreutils
          pkgs.findutils
          pkgs.gnused
          pkgs.gnugrep
          pkgs.ruby
          pkgs.bundler
        ];
    in
    {
      devShells = eachSystem (system: {
        default = nixpkgs.legacyPackages.${system}.mkShell {
          packages = tools nixpkgs.legacyPackages.${system};
          # nixpkgs' Go is the repo's toolchain; never download another one.
          GOTOOLCHAIN = "local";
        };
      });
      # The same tools as a plain bin/, for the job image's read-only store.
      packages = eachSystem (system: {
        default = nixpkgs.legacyPackages.${system}.buildEnv {
          name = "ci-tools";
          paths = tools nixpkgs.legacyPackages.${system};
          ignoreCollisions = true;
        };
      });
    };
}
