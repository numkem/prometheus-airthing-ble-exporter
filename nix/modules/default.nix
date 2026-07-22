{
  config,
  lib,
  pkgs,
  ...
}:

with lib;
let
  cfg = config.services.prometheus-airthings-ble-exporter;
in
{
  options.services.prometheus-airthings-ble-exporter = {
    enable = mkEnableOption "Enable the prometheus Airthings BLE exporter";

    package = mkOption {
      type = types.package;
      default = pkgs.prometheus-airthings-ble-exporter;
      description = mdDoc ''
        Package to use for the systemd service.
      '';
    };

    collectionDuration = mkOption {
      type = types.str;
      default = "30m";
      description = mdDoc ''
        Go duration for how often to prope the Wave for it's data
      '';
    };

    listenAddress = mkOption {
      type = types.str;
      default = "0.0.0.0";
      description = mdDoc ''
        Listening address for the exporter.

        The IP can be ommited for 0.0.0.0
      '';
    };

    port = mkOption {
      type = types.ints.u16;
      default = 9456;
      description = mdDoc ''
        Which port the exporter will listen on.
      '';
    };

    retries = lib.mkOption {
      type = lib.types.ints.unsigned;
      default = 3;
      description = "The number of retries contacting the Wave device.";
    };

    waveSerialNumber = mkOption {
      type = types.ints.unsigned;
      default = 0;
      description = mdDoc ''
        The serial number of the Airthings Wave to probe for data
      '';
    };
  };

  config = mkIf cfg.enable {
    systemd.services.prometheus-airthings-ble-exporter = {
      after = [ "networking.target" ];
      requires = [ "bluetooth.service" ];
      wantedBy = [ "multi-user.target" ];

      description = "Airthings Wave BLE exporter for prometheus";
      restartIfChanged = true;

      serviceConfig = {
        ExecStart = "${cfg.package}/bin/prometheus-airthings-ble-exporter -serial ${toString cfg.waveSerialNumber} -address ${cfg.listenAddress}:${toString cfg.port} -collection ${cfg.collectionDuration} -retries ${cfg.retries}";
        Restart = "always";

        LockPersonality = true;
        PrivateTmp = !config.boot.isContainer;
        PrivateUsers = true;
        ProtectControlGroups = !config.boot.isContainer;
        ProtectHostname = true;
        ProtectKernelLogs = !config.boot.isContainer;
        ProtectKernelModules = !config.boot.isContainer;
        ProtectKernelTunables = !config.boot.isContainer;
        RestrictNamespaces = !config.boot.isContainer;
        RestrictRealtime = true;
        RestrictSUIDSGID = true;
      };
    };
  };
}
