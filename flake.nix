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
      let
        version = builtins.replaceStrings [ "\n" ] [ "" ] (builtins.readFile ./VERSION);
      in
      {
        packages = {
          sgard = pkgs.buildGoModule rec {
            pname = "sgard";
            inherit version;
            src = pkgs.lib.cleanSource ./.;
            subPackages = [ "cmd/sgard" "cmd/sgardd" ];

            vendorHash = "sha256-Z/Ja4j7YesNYefQQcWWRG2v8WuIL+UNqPGwYD5AipZY=";

            ldflags = [ "-s" "-w" "-X main.version=${version}" ];

            meta = {
              description = "Shimmering Clarity Gardener: dotfile management";
              mainProgram = "sgard";
            };
          };

          sgard-fido2 = pkgs.buildGoModule rec {
            pname = "sgard-fido2";
            inherit version;
            src = pkgs.lib.cleanSource ./.;
            subPackages = [ "cmd/sgard" "cmd/sgardd" ];

            vendorHash = "sha256-Z/Ja4j7YesNYefQQcWWRG2v8WuIL+UNqPGwYD5AipZY=";

            buildInputs = [ pkgs.libfido2 ];
            nativeBuildInputs = [ pkgs.pkg-config ];
            tags = [ "fido2" ];

            ldflags = [ "-s" "-w" "-X main.version=${version}" ];

            meta = {
              description = "Shimmering Clarity Gardener: dotfile management (with FIDO2 hardware support)";
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
            libfido2
            pkg-config
          ];
        };
      }
    );
}
