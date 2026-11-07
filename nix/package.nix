{ lib
, stdenv
, python3
, nodejs_22
, buildNpmPackage
, writeShellApplication
,
}:

let
  version = (builtins.fromTOML (builtins.readFile ../cairn/pyproject.toml)).project.version;

  python = python3.withPackages (
    ps:
      with ps;
      [
        fastapi
        uvicorn
        click
        pyyaml
        docker
        requests
        uvloop
        httptools
        websockets
      ]
  );

  web = buildNpmPackage {
    pname = "cairn-web";
    inherit version;
    src = ../web;
    npmDepsHash = "sha256-XB5iukJZA1+2BiEhQ9gb6RbelABNsWfV6WxF0b4RvwA=";

    nativeBuildInputs = [ nodejs_22 ];

    buildPhase = ''
      runHook preBuild
      npm run build -- --outDir dist
      runHook postBuild
    '';

    installPhase = ''
      runHook preInstall
      mkdir -p $out
      cp -r dist/. $out/
      runHook postInstall
    '';
  };

  src = lib.cleanSourceWith {
    src = ../.;
    filter =
      path: type:
      let
        name = baseNameOf path;
      in
        !(builtins.elem name [
          ".git"
          ".venv"
          "node_modules"
          "dist"
          "datas"
          "result"
        ]);
  };

  app = stdenv.mkDerivation {
    pname = "cairn-app";
    inherit version src;

    installPhase = ''
      runHook preInstall
      mkdir -p $out
      cp -r cairn/src/cairn $out/cairn
      mkdir -p $out/cairn/server/static
      cp -r ${web}/. $out/cairn/server/static/
      runHook postInstall
    '';
  };
in
(writeShellApplication {
  name = "cairn";
  runtimeInputs = [ python ];
  text = ''
    export PYTHONPATH="${app}:''${PYTHONPATH:-}"
    exec python -c 'import sys; from cairn.cli import main; sys.exit(main())' "$@"
  '';
}).overrideAttrs
  (old: {
    passthru = (old.passthru or { }) // {
      inherit web app;
    };
    meta = {
      description = "Fact-graph based collaborative exploration protocol";
      homepage = "https://github.com/oritera/Cairn";
      license = lib.licenses.asl20;
      mainProgram = "cairn";
      platforms = lib.platforms.linux;
    };
  })
