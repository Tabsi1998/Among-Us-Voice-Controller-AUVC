using System.ComponentModel;
using System.Runtime.CompilerServices;
using AUVC.Transport;
using Config.Net;

[assembly: InternalsVisibleTo("DynamicProxyGenAssembly2")]

namespace AUCapture_WPF
{
    public interface IAppSettings : INotifyPropertyChanged
    {
        [Option(DefaultValue = "")]
        string language { get; set; }

        [Option(DefaultValue = true)]
        bool startupMemes { get; set; }

        [Option(DefaultValue = false)]
        bool alwaysOnTop { get; set; }

        [Option(DefaultValue = false)]
        bool ApiServer { get; set; }

        [Option(DefaultValue = false)]
        bool AlwaysCopyGameCode { get; set; }

        [Option(DefaultValue = "")]
        string SelectedAccent { get; set; }

        [Option(DefaultValue = false)]
        bool ranBefore { get; set; }

        [Option(DefaultValue = false)]
        bool DarkMode { get; set; }

        [Option(DefaultValue = false)]
        bool FocusOnToken { get; set; }

        [Option(DefaultValue = 18d)]
        double fontSize { get; set; }

        [Option(DefaultValue = false)]
        bool debug { get; set; }

        [Option(DefaultValue = true)]
        bool checkForUpdate { get; set; }

        // The address of the AUVC bot. The pairing code is deliberately not a
        // setting: it works once, and a used code has no business on disk.
        [Option(DefaultValue = BotAddress.Default)]
        string host { get; set; }
    }
}
