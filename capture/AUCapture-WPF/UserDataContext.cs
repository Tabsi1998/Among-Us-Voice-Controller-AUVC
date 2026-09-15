using ControlzEx.Theming;
using MahApps.Metro.Controls.Dialogs;
using System;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.ComponentModel;
using System.Diagnostics;
using System.Globalization;
using System.IO;
using System.Linq;
using System.Runtime.CompilerServices;
using System.Runtime.InteropServices;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Input;
using System.Windows.Media;
using System.Windows.Media.Imaging;
using AmongUsCapture;
using AUCapture_WPF.IPC;
using AUCapture_WPF.Models;
using AUVC.Transport;
using Humanizer;
using MahApps.Metro.Controls;
using Microsoft.Win32;
using Application = System.Windows.Application;

namespace AUCapture_WPF
{
    public class UserDataContext : INotifyPropertyChanged
    {
        public IDialogCoordinator DialogCoordinator { get; set; }
        public IAppSettings Settings { get; set; }

        public string Version { get; set; }

        private string _latestVersion = "";
        /// <summary>What the About tab says about the newest version.</summary>
        public string LatestVersion
        {
            get => _latestVersion;
            set
            {
                _latestVersion = value;
                OnPropertyChanged();
            }
        }

        private string _updateNotice = "";
        /// <summary>The line about a newer AUVC version, or empty when there is none to tell about.</summary>
        public string UpdateNotice
        {
            get => _updateNotice;
            set
            {
                _updateNotice = value;
                OnPropertyChanged();
            }
        }

        /// <summary>The page the line links to, always one of AUVC's release pages.</summary>
        public string UpdatePage { get; set; } = ReleaseFeed.ReleasesPage;
        private ICommand textBoxButtonCopyCmd;
        private ICommand openAmongUsCMD;
        private ICommand openLogFolderCMD;
        private ICommand copyLatestLogCMD;
        private ICommand openGuideCMD;
        private ICommand restartCMD;
        public List<AccentColorMenuData> AccentColors { get; set; }
        private bool? _connected = false;
        public bool? Connected
        {
            get => _connected;
            set
            {
                _connected = value;
                OnPropertyChanged();
            }
        }
        public class AccentColorMenuData
        {
            public string Name { get; set; }

            public Brush BorderColorBrush { get; set; }

            public Brush ColorBrush { get; set; }

            public AccentColorMenuData()
            {
                ChangeAccentCommand = new SimpleCommand
                {
                    CanExecuteDelegate = x => true,
                    ExecuteDelegate = DoChangeTheme
                };

            }

            public ICommand ChangeAccentCommand { get; }

            protected virtual void DoChangeTheme(object sender)
            {
                ThemeManager.Current.ChangeThemeColorScheme(System.Windows.Application.Current, Name);
            }
        }

        public ICommand TextBoxButtonCopyCmd => textBoxButtonCopyCmd ??= new SimpleCommand
        {
            CanExecuteDelegate = x =>
            {
                switch (x)
                {
                    case string s:
                        return s != "";
                    case TextBox t:
                        return t.Text != "";
                    case PasswordBox p:
                        return p.Password != "";
                    default:
                        return true;
                }
            },
            ExecuteDelegate = async x =>
            {
                if (x is string s)
                {
                    Clipboard.SetText(s);
                }
                else if (x is TextBox t)
                {
                    Clipboard.SetText(t.Text);
                }
                else if (x is PasswordBox p)
                {
                    Clipboard.SetText(p.Password);
                }
            }
        };
        private string GetAmongUsLauncherLink()
        {
            var key = Registry.CurrentUser.OpenSubKey("SOFTWARE\\Classes\\amongus\\shell\\open\\command");
            if (key is null)
            {
                return "steam://rungameid/945360";
            }
            else
            {
                var path = ((string)key.GetValue(""));
                path = path.Substring(0, path.Length - 4).Trim().Trim('\"');
                if (path.Contains("Epic Games"))
                {
                    return "com.epicgames.launcher://apps/963137e4c29d4c79a81323b8fab03a40?action=launch&silent=true";
                }
                else
                {
                    return "steam://rungameid/945360";
                }

            }

        }
        public ICommand OpenAmongUsCMD => openAmongUsCMD ??= new SimpleCommand
        {
            CanExecuteDelegate = x => true,
            ExecuteDelegate = async x =>
            {
                if (RuntimeInformation.IsOSPlatform(OSPlatform.Windows))
                {
                    Process.Start(new ProcessStartInfo(GetAmongUsLauncherLink()) { UseShellExecute = true });
                }
                else if (RuntimeInformation.IsOSPlatform(OSPlatform.Linux))
                {
                    Process.Start("xdg-open", GetAmongUsLauncherLink());
                }
                else if (RuntimeInformation.IsOSPlatform(OSPlatform.OSX))
                {
                    Process.Start("open", GetAmongUsLauncherLink());
                }
                else
                {
                    // throw 
                }
            }
        };
        public ICommand OpenLogFolderCmd => openLogFolderCMD ??= new SimpleCommand
        {
            CanExecuteDelegate = x => true,
            ExecuteDelegate = x =>
            {
                if (!Directory.Exists(App.LogFolder)) return;
                Process.Start(new ProcessStartInfo(App.LogFolder) { UseShellExecute = true });

            }
        };
        public ICommand RestartCmd => restartCMD ??= new SimpleCommand
        {
            CanExecuteDelegate = x => true,
            ExecuteDelegate = x =>
            {
                Application.Current.Invoke(() =>
                {
                    IPCAdapter.getInstance().mutex.ReleaseMutex(); //Release the mutex so the other app does not see us. 
                    ProcessStartInfo startInfo = new ProcessStartInfo(Process.GetCurrentProcess().MainModule.FileName);
                    if (StartToken.LastRawToken is not null)
                    {
                        startInfo.Arguments = $"\"{StartToken.LastRawToken}\"";
                    }

                    Process.Start(startInfo);
                    Application.Current.Shutdown(0);
                });
            }
        };
        /// <summary>F1: the AUVC guide, in the language the app shows.</summary>
        public ICommand OpenGuideCmd => openGuideCMD ??= new SimpleCommand
        {
            CanExecuteDelegate = x => true,
            ExecuteDelegate = x =>
            {
                OpenBrowser(AppLanguage.GuideFor(CultureInfo.CurrentUICulture));
            }
        };
        public ICommand CopyLatestLogCMD => copyLatestLogCMD ??= new SimpleCommand
        {
            CanExecuteDelegate = x => true,
            ExecuteDelegate = x =>
            {
                if (!Directory.Exists(App.LogFolder)) return;
                if (!File.Exists(Path.Join(App.LogFolder, "latest.log"))) return;
                string logText = File.ReadAllText(Path.Join(App.LogFolder, "latest.log"));
                if (logText.Length <= 1994)
                {
                    logText = $"```{logText}```";
                }
                Clipboard.SetText(logText);
            }
        };
        private ObservableCollection<Player> _players = new ObservableCollection<Player>();
        public ObservableCollection<Player> Players
        {
            get => _players;
            set
            {
                _players = value;
                OnPropertyChanged();
            }
        }
        private static void OpenBrowser(string url)
        {
            if (RuntimeInformation.IsOSPlatform(OSPlatform.Windows))
            {
                Process.Start(new ProcessStartInfo(url) { UseShellExecute = true });
            }
            else if (RuntimeInformation.IsOSPlatform(OSPlatform.Linux))
            {
                Process.Start("xdg-open", url);
            }
            else if (RuntimeInformation.IsOSPlatform(OSPlatform.OSX))
            {
                Process.Start("open", url);
            }
        }
        private ObservableCollection<ConnectionStatus> _connectionStatuses = new ObservableCollection<ConnectionStatus>();
        public ObservableCollection<ConnectionStatus> ConnectionStatuses
        {
            get => _connectionStatuses;
            set
            {
                _connectionStatuses = value;
                OnPropertyChanged();
            }
        }

        private PlayMap? _gameMap;
        public PlayMap? GameMap
        {
            get => _gameMap;
            set
            {
                _gameMap = value;
                OnPropertyChanged();
            }
        }

        private string _gameCode;
        public string GameCode
        {
            get => _gameCode;
            set
            {
                _gameCode = value;
                OnPropertyChanged();
            }
        }

        private Brush _backgroundBrush = new ImageBrush(new BitmapImage(new Uri("pack://application:,,,/Resources/Misc/AutoBG.png")));
        public Brush BackgroundBrush
        {
            get => _backgroundBrush;
            set
            {
                _backgroundBrush = value;
                OnPropertyChanged();
            }
        }

        private int _playerRows = 2;
        public int PlayerRows
        {
            get => _playerRows;
            set
            {
                _playerRows = value;
                OnPropertyChanged();
            }
        }

        private int _playerCols = 2;
        public int PlayerCols
        {
            get => _playerCols;
            set
            {
                _playerCols = value;
                OnPropertyChanged();
            }
        }

        private GameState? _gameState;
        public GameState? GameState
        {
            get => _gameState;
            set
            {
                _gameState = value;
                OnPropertyChanged();
            }
        }

        private string _statusIcon = "";
        public string StatusIcon
        {
            get => _statusIcon;
            set
            {
                _statusIcon = value;
                OnPropertyChanged();
            }
        }

        private string _statusHeadline = "";
        public string StatusHeadline
        {
            get => _statusHeadline;
            set
            {
                _statusHeadline = value;
                OnPropertyChanged();
            }
        }

        private string _statusNextStep = "";
        public string StatusNextStep
        {
            get => _statusNextStep;
            set
            {
                _statusNextStep = value;
                OnPropertyChanged();
            }
        }

        private static void Shuffle<T>(List<T> list)
        {
            Random rng = new Random();
            int n = list.Count;
            while (n > 1)
            {
                n--;
                int k = rng.Next(n + 1);
                T value = list[k];
                list[k] = list[n];
                list[n] = value;
            }
        }
        public void GeneratePlayers(int numOfPlayers)
        {
            var nums = Enumerable.Range(0, 12).ToList();
            Shuffle(nums);
            //var colors  = nums.Cast<PlayerColor>().Where(x=>!Players.Select(y=>y.Color).Contains(x)).Take(numOfPlayers).ToList();
            //foreach (var color in colors)
            //{
            //    var newPlayer = new Player(color.Humanize(), color, true);
            //    Players.Add(newPlayer);
            //}
        }
        public UserDataContext(IDialogCoordinator dialogCoordinator, IAppSettings settings)
        {
            DialogCoordinator = dialogCoordinator;
            Settings = settings;
            Settings.debug = AmongUsCapture.Settings.PersistentSettings.debugConsole;
            Settings.PropertyChanged += SettingsOnPropertyChanged;
            AccentColors = ThemeManager.Current.Themes
                .GroupBy(x => x.ColorScheme)
                .OrderBy(a => a.Key)
                .Select(a => new AccentColorMenuData { Name = a.Key, ColorBrush = a.First().ShowcaseBrush })
                .ToList();
            // The newest version is looked up by the main window, after it is shown:
            // a network request must not hold up the start.
            Version = ReleaseCheck.Current?.ToString() ?? "";
            OnPropertyChanged(nameof(Version));
            OnPropertyChanged(nameof(AccentColors));



        }

        private void SettingsOnPropertyChanged(object sender, PropertyChangedEventArgs e)
        {
            if (e.PropertyName == nameof(Settings.debug))
            {
                AmongUsCapture.Settings.PersistentSettings.debugConsole = Settings.debug;
                Task.Factory.StartNew((() =>
                {
                    var selection = this.DialogCoordinator.ShowMessageAsync(this, Properties.Resources.RestartRequiredTitle,
                        Settings.debug ? Properties.Resources.DebugModeOnRestart : Properties.Resources.DebugModeOffRestart,
                        MessageDialogStyle.AffirmativeAndNegative,
                        new MetroDialogSettings
                        {
                            AnimateHide = true,
                            AffirmativeButtonText = Properties.Resources.RestartText,
                            NegativeButtonText = Properties.Resources.LaterText,
                            DefaultButtonFocus = MessageDialogResult.Affirmative,
                        }).Result;
                    if (selection == MessageDialogResult.Affirmative)
                    {
                        Application.Current.Invoke(() =>
                        {
                            IPCAdapter.getInstance().mutex.ReleaseMutex(); //Release the mutex so the other app does not see us. 
                            Process.Start(Process.GetCurrentProcess().MainModule.FileName);
                            Application.Current.Shutdown(0);
                        });
                    }

                }));


            }
        }

        public event PropertyChangedEventHandler? PropertyChanged;


        protected void OnPropertyChanged([CallerMemberName] string propertyName = null)
        {
            PropertyChanged?.Invoke(this, new PropertyChangedEventArgs(propertyName));
        }
    }
}