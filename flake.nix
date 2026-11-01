{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs { inherit system; };
        nativeBuildInputs = with pkgs; [ ];
        buildInputs = with pkgs; [ ];
      in
      {
        formatter = pkgs.nixfmt-tree;

        packages.default = pkgs.buildGoLatestModule {
          inherit buildInputs nativeBuildInputs;
          pname = "tsmvproxy";
          version = "0.1.0";
          src = ./.;
          vendorHash = "sha256-vpY2kJWzHsqx5sp5ZXL2TGQNc9imuVwKQ6u7yMMSMv8=";
        };

        packages.container = pkgs.dockerTools.buildImage {
          name = "ghcr.io/jess-sch/tsmvproxy";
          tag = "latest";
          created = "now";
          copyToRoot = with pkgs; [ iana-etc cacert self.packages."${system}".default ];
          runAsRoot = ''
            mkdir -p /data
          '';
          config = {
            Entrypoint = [ "/bin/tsmvproxy" "--state-dir" "/data" ];
            Volumes."/data" = {};
          };
        };

        devShells.default = pkgs.mkShell {
          hardeningDisable = [ "fortify" ];
          packages =
            (with pkgs; [
              go_latest
              gopls
              gotools
              delve
            ])
            ++ buildInputs
            ++ nativeBuildInputs;
        };
      }
    );
}
