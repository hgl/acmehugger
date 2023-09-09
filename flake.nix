{
  inputs.nixpkgs.url = "nixpkgs";
  inputs.utils.url = "flake-utils";

  outputs = { self, nixpkgs, utils }:
    utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        devShells.default = pkgs.mkShell
          {
            packages = with pkgs; [
              go_1_21
              gopls
              gotools
              go-tools
              nil
              nodePackages.dockerfile-language-server-nodejs
              nginx
            ];
          };
      });
}
