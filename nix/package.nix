{
  lib,
  stdenv,
  nodejs_22,
  buildGoModule,
  buildNpmPackage,
}:

let
  version = "0.2.1";

  dispatcher = buildGoModule {
    pname = "cairn-dispatcher";
    inherit version;
    src = ../dispatcher;
    vendorHash = "sha256-Zj8VxAA11BXxvZUf9YewHkEEcI24sS8vPu22iu4LdWc=";
    subPackages = [ "cmd/cairn-dispatcher" ];
    ldflags = [ "-s" "-w" ];
    meta = {
      description = "Cairn dispatcher";
      mainProgram = "cairn-dispatcher";
    };
  };

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

  server = buildGoModule {
    pname = "cairn-server";
    inherit version;
    src = ../server;
    vendorHash = "sha256-4IjZh0PR9LEGsc8QucR8XxsaJuht1c8MDk/nN8qR6+A=";
    subPackages = [ "cmd/cairn" ];
    ldflags = [ "-s" "-w" ];

    preBuild = ''
      rm -rf internal/api/static
      mkdir -p internal/api/static
      cp -r ${web}/. internal/api/static/
    '';

    meta = {
      description = "Cairn API server";
      mainProgram = "cairn";
    };
  };
in
stdenv.mkDerivation {
  pname = "cairn";
  inherit version;
  dontUnpack = true;

  installPhase = ''
    runHook preInstall
    mkdir -p $out/bin
    install -m755 ${server}/bin/cairn $out/bin/cairn
    install -m755 ${dispatcher}/bin/cairn-dispatcher $out/bin/cairn-dispatcher
    runHook postInstall
  '';

  passthru = {
    inherit web server dispatcher;
  };

  meta = {
    description = "Fact-graph based collaborative exploration protocol";
    homepage = "https://github.com/oritera/Cairn";
    license = lib.licenses.asl20;
    mainProgram = "cairn";
    platforms = lib.platforms.linux;
  };
}
