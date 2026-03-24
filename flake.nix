{
  description = "sgard — Shimmering Clarity Gardener: dotfile management";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
      in
      {
        packages = {
          sgard = pkgs.buildGoModule {
            pname = "sgard";
            version = "0.1.0";
            src = pkgs.lib.cleanSource ./.;
            subPackages = [ "cmd/sgard" ];

            vendorHash = "sha256-uJMkp08SqZaZ6d64Li4Tx8I9OYjaErLexBrJaf6Vb60=";

            ldflags = [ "-s" "-w" ];

            meta = {
              description = "Shimmering Clarity Gardener: dotfile management";
              mainProgram = "sgard";
            };
          };

          default = self.packages.${system}.sgard;
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            golangci-lint
            protobuf
            protoc-gen-go
            protoc-gen-go-grpc
          ];
        };
      }
    );
}
