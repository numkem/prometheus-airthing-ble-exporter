{
  config,
  lib,
  pkgs,
  ...
}:

with lib;
let
  cfg = config.services.prometheus.exporters.airthing-ble;
in
{
  options.services.prometheus.exporters.airthing-ble = {
    enable = mkEnableOption "Enable the prometheus Airthing BLE exporter";

    package = mkOption {
      type = types.package;
      default = pkgs.prometheus-airthing-ble-exporter;
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

    waveSerialNumber = mkOption {
      type = types.ints.unsigned;
      default = 0;
      description = mdDoc ''
        The serial number of the Airthing Wave to probe for data
      '';
    };

  };

  config = mkIf cfg.enable {
    systemd.services.prometheus-airthing-ble-exporter = {
      description = "Airthing Wave BLE exporter for prometheus";
      restartIfChanged = true;

      serviceConfig.ExecStart = "${cfg.package}/bin/prometheus-airthing-ble-exporter -serial ${cfg.waveSerialNumber} -address ${cfg.listenAddress}:${cfg.port} -collection ${cfg.collectionDuration}";

      ProtectHostname = true;
      PrivateTmp = !config.boot.isContainer;
      PrivateUsers = true;
    };
  };
}
