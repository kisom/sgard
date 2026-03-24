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
            version = "2.0.0";
            src = pkgs.lib.cleanSource ./.;
            subPackages = [ "cmd/sgard" "cmd/sgardd" ];

            vendorHash = "sha256-6tLNIknbxrRWYKo5x7yMX6+JDJxbF5l2WBIxXaF7OZ4=";

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
