Ext.namespace("SYNO.SDS.ActiveBackupZabbix");
Ext.namespace("SYNO.SDS.ActiveBackupZabbix.Utils");

Ext.apply(SYNO.SDS.ActiveBackupZabbix.Utils, function() {
  return {
    getSynoToken: function() {
      var values = [];
      try {
        if (SYNO && SYNO.SDS && SYNO.SDS.Session) {
          if (typeof SYNO.SDS.Session.get === "function") {
            values.push(SYNO.SDS.Session.get("SynoToken"));
            values.push(SYNO.SDS.Session.get("synotoken"));
            values.push(SYNO.SDS.Session.get("synoToken"));
          }
          values.push(SYNO.SDS.Session.SynoToken);
          values.push(SYNO.SDS.Session.synotoken);
          values.push(SYNO.SDS.Session.synoToken);
        }
      } catch (ignored) {}
      try {
        values.push(window.SynoToken);
        values.push(window.synotoken);
      } catch (ignored) {}

      for (var i = 0; i < values.length; i += 1) {
        if (typeof values[i] === "string" && values[i].length > 0) {
          return values[i];
        }
      }
      return "";
    },

    getMainHtml: function() {
      var params = "_ts=" + new Date().getTime();
      var synoToken = this.getSynoToken();
      if (synoToken) {
        params += "&SynoToken=" + encodeURIComponent(synoToken);
      }
      var url = "webman/3rdparty/synology-activebackup-zabbix/web/index.html?" + params;
      return '<iframe src="' + url + '" title="Active Backup Zabbix" style="display:block;width:100%;height:100%;border:0;margin:0;padding:0;background:#f4f6fa;"></iframe>';
    }
  };
}());

Ext.define("SYNO.SDS.ActiveBackupZabbix.Application", {
  extend: "SYNO.SDS.AppInstance",
  appWindowName: "SYNO.SDS.ActiveBackupZabbix.MainWindow",

  constructor: function() {
    this.callParent(arguments);
  }
});

Ext.define("SYNO.SDS.ActiveBackupZabbix.MainWindow", {
  extend: "SYNO.SDS.AppWindow",

  constructor: function(config) {
    var app = SYNO.SDS.ActiveBackupZabbix;
    config = config || {};
    this.appInstance = config.appInstance;

    app.MainWindow.superclass.constructor.call(this, Ext.apply({
      title: "Active Backup Zabbix",
      layout: "fit",
      width: 1180,
      height: 760,
      minWidth: 820,
      minHeight: 520,
      resizable: true,
      maximizable: true,
      minimizable: true,
      html: app.Utils.getMainHtml()
    }, config));
  },

  onOpen: function() {
    SYNO.SDS.ActiveBackupZabbix.MainWindow.superclass.onOpen.apply(this, arguments);
  },

  onRequest: function(request) {
    SYNO.SDS.ActiveBackupZabbix.MainWindow.superclass.onRequest.call(this, request);
  },

  onClose: function() {
    SYNO.SDS.ActiveBackupZabbix.MainWindow.superclass.onClose.apply(this, arguments);
    this.doClose();
    return true;
  }
});
