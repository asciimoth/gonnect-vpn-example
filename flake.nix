{
  description = "Gonnect example VPN";
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    flake-utils = {
      url = "github:numtide/flake-utils";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    pre-commit-hooks = {
      url = "github:cachix/pre-commit-hooks.nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };
  outputs = {
    self,
    nixpkgs,
    flake-utils,
    pre-commit-hooks,
    ...
  }:
    flake-utils.lib.eachDefaultSystem (system: let
      pkgs = import nixpkgs {
        inherit system;
        config.allowUnfree = true;
        config.android_sdk.accept_license = true;
      };
      androidPkgs = pkgs.androidenv.composeAndroidPackages {
        platformVersions = [ "35" ];
        buildToolsVersions = [ "35.0.0" ];
        includeNDK = true;
        includeCmake = true;
      };

      checks = {
        pre-commit-check = pre-commit-hooks.lib.${system}.run {
          src = ./.;
          hooks = {
            gotest.enable = true;
            commitizen.enable = true;
            typos.enable = true;
            typos-commit = {
              enable = true;
              description = "Find typos in commit message";
              entry = let script = pkgs.writeShellScript "typos-commit" ''
                typos "$1"
              ''; in builtins.toString script;
              stages = [ "commit-msg" ];
            };
            govet.enable = true;
            gofmt.enable = true;
            golangci-lint.enable = true;
            gotidy = {
              enable = true;
              description = "Makes sure go.mod matches the source code";
              entry = let script = pkgs.writeShellScript "gotidyhook" ''
                go mod tidy -v
              ''; in builtins.toString script;
              stages = [ "pre-commit" ];
            };
          };
        };
      };
    in {
      devShells.default = pkgs.mkShell {
        shellHook = checks.pre-commit-check.shellHook + ''
          export JAVA_HOME=${pkgs.openjdk17_headless}
          export ANDROID_HOME=${androidPkgs.androidsdk}/libexec/android-sdk
          export ANDROID_SDK_ROOT=$ANDROID_HOME
          export ANDROID_NDK_HOME=${androidPkgs.ndk-bundle}
          export ANDROID_NDK_ROOT=${androidPkgs.ndk-bundle}
        '';
        nativeBuildInputs = with pkgs; [
          pkg-config
        ];
        buildInputs = with pkgs; [
          go
          gomobile
          golangci-lint
          gopls

          just

          gradle
          openjdk17_headless
          android-tools

          typos
          commitizen

          pkgsite
          pkg-config

          androidPkgs.androidsdk
          androidPkgs.platform-tools
          androidPkgs.ndk-bundle

          # graphics / windowing
          mesa
          libGL
          vulkan-headers
          vulkan-loader

          # wayland
          wayland
          wayland-protocols

          # x11
          xorg.libX11
          xorg.libXcursor
          xorg.libXi
          xorg.libXrandr
          xorg.libXfixes
          xorg.libxcb

          # keyboard
          libxkbcommon
        ] ++ androidPkgs.build-tools ++ androidPkgs.cmake;
      };
    });
}
