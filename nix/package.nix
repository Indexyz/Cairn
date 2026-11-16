{ lib
, stdenv
, python3
, nodejs_22
, buildGoModule
, buildNpmPackage
, writeShellApplication
,
}:

let
  version = (builtins.fromTOML (builtins.readFile ../cairn/pyproject.toml)).project.version;

  dispatcher = buildGoModule {
    pname = "cairn-dispatcher";
    inherit version;
    src = ../dispatcher;
    vendorHash = "sha256-Zj8VxAA11BXxvZUf9YewHkEEcI24sS8vPu22iu4LdWc=";
    subPackages = [ "cmd/cairn-dispatcher" ];
    ldflags = [ "-s" "-w" ];
    meta = {
      description = "Cairn dispatcher (Go)";
      mainProgram = "cairn-dispatcher";
    };
  };

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
    npmDepsHash = "sha256-zVYIaeo9CWiKZDxTQex6xo5sdtGec4kXsdGhJGGVihI=";

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

  cairnScript = writeShellApplication {
    name = "cairn";
    runtimeInputs = [ python ];
    text = ''
      export PYTHONPATH="${app}:''${PYTHONPATH:-}"
      exec python -c 'import sys; from cairn.cli import main; sys.exit(main())' "$@"
    '';
  };
in
stdenv.mkDerivation {
  pname = "cairn";
  inherit version;
  dontUnpack = true;

  installPhase = ''
    runHook preInstall
    mkdir -p $out/bin
    install -m755 ${dispatcher}/bin/cairn-dispatcher $out/bin/cairn-dispatcher
    install -m755 ${cairnScript}/bin/cairn $out/bin/cairn
    runHook postInstall
  '';

  passthru = {
    inherit web app dispatcher;
  };

  meta = {
    description = "Fact-graph based collaborative exploration protocol";
    homepage = "https://github.com/oritera/Cairn";
    license = lib.licenses.asl20;
    mainProgram = "cairn";
    platforms = lib.platforms.linux;
  };
}
