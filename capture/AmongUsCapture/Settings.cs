using System;
using System.IO;
using System.Windows;
using Config.Net;

namespace AmongUsCapture
{
    public static class Settings
    {
        public static string StorageLocation = Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "AmongUsCapture");

        //Global persistent settings that are saved to a json file. Limited Types
        public static IPersistentSettings PersistentSettings;

        static Settings()
        {
            reloadSettings();
        }

        public static void reloadSettings()
        {
            try
            {
                PersistentSettings = new ConfigurationBuilder<IPersistentSettings>().UseJsonFile(Path.Join(StorageLocation, "Settings.json")).Build();

            }
            catch (Newtonsoft.Json.JsonReaderException e) //Delete file and recreate config
            {
                Console.WriteLine($"Bad config. Clearing.");
                File.Delete(Path.Join(StorageLocation, "Settings.json"));
                PersistentSettings = new ConfigurationBuilder<IPersistentSettings>().UseJsonFile(Path.Join(StorageLocation, "Settings.json")).Build();
            }
        }

    }


    public interface IPersistentSettings
    {
        //Types allowed: bool, double, int, long, string, TimeSpan, DateTime, Uri, Guid
        //DateTime is always converted to UTC
        [Option(Alias = "Host", DefaultValue = "http://localhost:8123")]
        string host { get; set; }

        [Option(Alias = "DebugConsole", DefaultValue = false)]
        bool debugConsole { get; set; }

        [Option(Alias = "IndexURL", DefaultValue = "")]
        string IndexURL { get; set; }
    }
}
