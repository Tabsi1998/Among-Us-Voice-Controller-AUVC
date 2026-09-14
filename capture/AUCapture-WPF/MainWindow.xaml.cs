using AmongUsCapture;
using AUCapture_WPF.IPC;
using Config.Net;
using ControlzEx.Theming;
using MahApps.Metro.Controls;
using MahApps.Metro.Controls.Dialogs;
using Newtonsoft.Json;
using Newtonsoft.Json.Converters;
using System;
using System.Collections;
using System.Collections.Generic;
using System.Collections.Specialized;
using System.ComponentModel;
using System.Diagnostics;
using System.Diagnostics.Eventing.Reader;
using System.Globalization;
using System.IO;
using System.IO.Compression;
using System.Linq;
using System.Net;
using System.Reflection;
using System.Runtime.CompilerServices;
using System.Runtime.InteropServices;
using System.Security.Cryptography;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Documents;
using System.Windows.Input;
using System.Windows.Media;
using System.Windows.Media.Imaging;
using System.Windows.Threading;
using AUCapture_WPF.Models;
using AUCapture_WPF.Properties;
using AUVC.Transport;
using Gu.Localization;
using HandyControl.Tools;
using HandyControl.Tools.Extension;
using Humanizer;
using Microsoft.Win32;
using NLog;
using Color = System.Drawing.Color;
using PlayerColor = AmongUsCapture.PlayerColor;

namespace AUCapture_WPF
{
    /// <summary>
    ///     Interaction logic for MainWindow.xaml
    /// </summary>
    public partial class MainWindow
    {
        public Color NormalTextColor = Color.White;
        private static readonly NLog.Logger Logger = NLog.LogManager.GetCurrentClassLogger();
        private readonly IAppSettings config;

        public UserDataContext context;
        private readonly bool connected;
        private readonly object locker = new();
        private readonly Queue<Player> DeadMessages = new();
        private Task ThemeGeneration;

        public MainWindow()
        {
            InitializeComponent();

            try
            {
                config = new ConfigurationBuilder<IAppSettings>()
                    .UseJsonFile(Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData),
                        "\\AmongUsCapture\\AmongUsGUI", "Settings.json")).Build();
            }
            catch (JsonReaderException e) //Delete file and recreate config
            {
                Console.WriteLine("Bad config. Clearing.");
                File.Delete(Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "\\AmongUsCapture\\AmongUsGUI", "Settings.json"));
                config = new ConfigurationBuilder<IAppSettings>()
                    .UseJsonFile(Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData),
                        "\\AmongUsCapture\\AmongUsGUI", "Settings.json")).Build();
            }

            context = new UserDataContext(DialogCoordinator.Instance, config);
            DataContext = context;
            // Before anything shows text, and again whenever the settings change it.
            context.Settings.language = AppLanguage.Normalize(context.Settings.language);
            ApplyLanguage();
            context.Settings.PropertyChanged += (_, e) =>
            {
                if (e.PropertyName == nameof(IAppSettings.language)) ApplyLanguage();
            };
            context.ConnectionStatuses.Add(new ConnectionStatus { Connected = false, ConnectionName = BotConnectionName });
            if (context.Settings.runBotOnThisPc)
            {
                SetLocalBotStatus(false);
            }
            UpdateSetupButton();
            Window.Topmost = context.Settings.alwaysOnTop;
            GameMemReader.getInstance().GameStateChanged += GameStateChangedHandler;
            GameMemReader.getInstance().ProcessHook += OnProcessHook;
            GameMemReader.getInstance().PlayerChanged += UserForm_PlayerChanged;
            GameMemReader.getInstance().PlayerCosmeticChanged += OnPlayerCosmeticChanged;
            GameMemReader.getInstance().CrackDetected += OnCrackDetected;
            GameMemReader.getInstance().JoinedLobby += OnJoinedLobby;
            GameMemReader.getInstance().GameOver += OnGameOver;
            BotConnection.Link.StatusChanged += OnLinkStatusChanged;
            context.Players.CollectionChanged += PlayersOnCollectionChanged;

            IPCAdapter.getInstance().OnToken += (sender, token) =>
            {
                // An aucapture:// link carries the address of the bot and a pairing code.
                Dispatcher.InvokeAsync(() => PairAsync(token.Host, token.ConnectCode));

                this.BeginInvoke(w =>
                {
                    if (!w.context.Settings.FocusOnToken) return;

                    if (w.WindowState.Equals(WindowState.Minimized)) w.WindowState = WindowState.Normal;

                    w.Show();
                    w.Activate();
                    w.Focus(); // important
                });
            };

            context.Players.CollectionChanged += PlayersOnCollectionChanged;

            //ApplyDarkMode();
        }

        private void OnCrackDetected(object? sender, EventArgs e)
        {
            var x = context.DialogCoordinator.ShowMessageAsync(context, Properties.Resources.CrackDetectedTitle, Properties.Resources.CrackDetectedMessage, MessageDialogStyle.AffirmativeAndNegative,
                new MetroDialogSettings
                {
                    AffirmativeButtonText = Properties.Resources.ContinueText,
                    NegativeButtonText = Properties.Resources.ExitText,
                    ColorScheme = MetroDialogColorScheme.Theme,
                    DefaultButtonFocus = MessageDialogResult.Negative
                }).ConfigureAwait(false).GetAwaiter().GetResult();
            if (x == MessageDialogResult.Negative)
            {
                Environment.Exit(0);
            }
            else
            {
                GameMemReader.getInstance().cracked = false;
            }

        }

        // The display language of Windows, read before the app replaces it with its own.
        private readonly CultureInfo windowsLanguage = CultureInfo.CurrentUICulture;

        // One language for everything: the resource texts, through the translator
        // and the thread culture, and SetupText, which reads the translator.
        private void ApplyLanguage()
        {
            var language = AppLanguage.Resolve(context.Settings.language, windowsLanguage);
            CultureInfo.DefaultThreadCurrentUICulture = language;
            CultureInfo.CurrentUICulture = language;
            Translator.Culture = language;
            UpdateSetupButton();
        }


        private void PlayersOnCollectionChanged(object? sender, NotifyCollectionChangedEventArgs e)
        {
            context.PlayerRows = (int)Math.Ceiling(Math.Sqrt(context.Players.Count));
            context.PlayerCols = (int)Math.Ceiling(context.Players.Count / Math.Ceiling(Math.Sqrt(context.Players.Count)));
            Trace.WriteLine(context.PlayerCols);
            Trace.WriteLine(context.PlayerRows);
        }

        private void OnPlayerCosmeticChanged(object? sender, PlayerCosmeticChangedEventArgs e)
        {
            if (context.Players.Any(x => x.Name == e.Name))
            {
                var player = context.Players.First(x => x.Name == e.Name);
                Console.WriteLine("Cosmetic change " + JsonConvert.SerializeObject(e));
                Dispatcher.Invoke(() =>
                {
                    player.HatID = e.HatId;
                    player.PantsID = e.SkinId;
                    player.PetID = e.PetId;
                });
            }
        }

        private const string BotConnectionName = "AUVC bot";
        private const string LocalBotConnectionName = "Discord bot";

        // Set once a refused credential has been replaced from the bot on this PC.
        // Once per run: a capture revoked on purpose must not keep coming back.
        private bool replacedRefusedCredential;

        private void OnLinkStatusChanged(LinkStatus status)
        {
            Dispatcher.InvokeAsync(async () =>
            {
                context.ConnectionStatuses.First(x => x.ConnectionName == BotConnectionName).Connected =
                    status.State == LinkState.Connected;

                // Every other state resolves itself. A refusal does not, and nothing
                // else would tell the person running capture what to do about it.
                if (status.State != LinkState.Refused) return;

                // The bot on this PC can issue a new credential itself, so a refused one
                // is replaced rather than reported.
                if (context.Settings.runBotOnThisPc && LocalBot.IsRunning && !replacedRefusedCredential &&
                    status.Code == AUVC.Protocol.ProtocolContract.CodeUnauthenticated)
                {
                    replacedRefusedCredential = true;
                    try
                    {
                        await LocalBot.ConnectCaptureAsync(context.Settings.botGuildId, freshCredential: true);
                        return;
                    }
                    catch (Exception error) when (error is LocalControlException or System.Net.Http.HttpRequestException or InvalidOperationException)
                    {
                        Logger.Warn("Could not replace the refused credential: {message}", error.Message);
                    }
                }

                var next = status.Code == AUVC.Protocol.ProtocolContract.CodeIncompatibleProtocol
                    ? Properties.Resources.CaptureRefusedIncompatible
                    : Properties.Resources.CaptureRefusedNewCode;
                await this.ShowMessageAsync(Properties.Resources.CaptureRefusedTitle,
                    status.Detail + Environment.NewLine + Environment.NewLine + next);
            });
        }

        private void SetLocalBotStatus(bool connected)
        {
            var status = context.ConnectionStatuses.FirstOrDefault(x => x.ConnectionName == LocalBotConnectionName);
            if (status is null)
            {
                context.ConnectionStatuses.Add(new ConnectionStatus { Connected = connected, ConnectionName = LocalBotConnectionName });
                return;
            }
            status.Connected = connected;
        }

        // Once the bot is set up, the same button opens its settings instead of the
        // first-run steps, so anything chosen during setup can be changed later.
        private void SetupButton_Click(object sender, RoutedEventArgs e) => OpenSetup(editing: context.Settings.runBotOnThisPc);

        private void UpdateSetupButton()
        {
            SetupButton.Content = context.Settings.runBotOnThisPc ? SetupText.SettingsButton : SetupText.SetupButton;
            SetupButton.ToolTip = context.Settings.runBotOnThisPc ? SetupText.SettingsButtonTooltip : SetupText.SetupButtonTooltip;
        }

        private void OpenSetup(bool editing = false)
        {
            var setup = new SetupWindow(context.Settings, editing) { Owner = this };
            setup.ShowDialog();
            UpdateSetupButton();

            if (context.Settings.runBotOnThisPc)
            {
                SetLocalBotStatus(LocalBot.IsRunning);
            }
            else
            {
                var local = context.ConnectionStatuses.FirstOrDefault(x => x.ConnectionName == LocalBotConnectionName);
                if (local is not null)
                {
                    context.ConnectionStatuses.Remove(local);
                }
            }
        }

        /// <summary>
        /// Starts the bot when this PC runs it, and offers the setup on a first start
        /// with nothing set up at all.
        /// </summary>
        private async Task StartLocalBotOrOfferSetupAsync()
        {
            if (!context.Settings.runBotOnThisPc)
            {
                if (!BotConnection.IsPaired)
                {
                    OpenSetup();
                }
                return;
            }

            try
            {
                await LocalBot.StartAsync();
                SetLocalBotStatus(true);
                await LocalBot.ConnectCaptureAsync(context.Settings.botGuildId, freshCredential: false);
            }
            catch (Exception error) when (error is BotHostException or BotTokenException or LocalControlException
                                              or System.Net.Http.HttpRequestException or InvalidOperationException)
            {
                SetLocalBotStatus(false);
                var message = error is BotHostException hostError
                    ? SetupText.BotStartFailed(hostError) + (hostError.LogTail.Length > 0 ? Environment.NewLine + Environment.NewLine + hostError.LogTail : "")
                    : error.Message;
                var answer = await this.ShowMessageAsync(SetupText.BotFailedTitle, message, MessageDialogStyle.AffirmativeAndNegative,
                    new MetroDialogSettings { AffirmativeButtonText = SetupText.OpenSetup, NegativeButtonText = SetupText.Close });
                if (answer == MessageDialogResult.Affirmative)
                {
                    OpenSetup(editing: true);
                }
            }
        }

        private async Task PairAsync(string address, string code)
        {
            try
            {
                var outcome = await BotConnection.PairAsync(address, code);
                context.Settings.host = outcome.Address.ToString();
                Code.Text = "";
                ManualConnectionFlyout.IsOpen = false;
                await this.ShowMessageAsync(Properties.Resources.PairedTitle, outcome.Message);
            }
            catch (PairingRefusedException refused)
            {
                await this.ShowMessageAsync(Properties.Resources.PairingFailedTitle, refused.Message);
            }
        }

        private void OnProcessHook(object? sender, ProcessHookArgs e)
        {
            context.Connected = true;
            //context.ConnectionStatuses.First(x => x.ConnectionName == "Among us").Connected = true;
            ProcessMemory.getInstance().process.Exited += ProcessOnExited;
        }

        private void ProcessOnExited(object? sender, EventArgs e)
        {
            Dispatcher.Invoke(() =>
            {
                context.Connected = false;
                //context.ConnectionStatuses.First(x => x.ConnectionName == "Among us").Connected = false;
            });
            ProcessMemory.getInstance().process.Exited -= ProcessOnExited;
        }


        private void OnGameOver(object? sender, GameOverEventArgs e)
        {
            Dispatcher.Invoke(() =>
            {
                foreach (var player in context.Players) player.Alive = true;
            });
        }


        private void UserForm_PlayerChanged(object sender, PlayerChangedEventArgs e)
        {
            if (e.Action == PlayerAction.Died)
            {
                if (context.Players.Any(x => x.Name == e.Name)) DeadMessages.Enqueue(context.Players.First(x => x.Name == e.Name));
            }
            else
            {
                if (e.Action != PlayerAction.Joined && context.Players.Any(x => string.Equals(x.Name, e.Name, StringComparison.CurrentCultureIgnoreCase)))
                {
                    var player = context.Players.First(x => string.Equals(x.Name, e.Name, StringComparison.CurrentCultureIgnoreCase));
                    Dispatcher.Invoke(() =>
                    {
                        switch (e.Action)
                        {
                            case PlayerAction.ChangedColor:
                                player.Color = e.Color;
                                break;

                            case PlayerAction.Disconnected:
                            case PlayerAction.Left:
                                context.Players.Remove(player);
                                break;

                            case PlayerAction.Exiled:
                            case PlayerAction.Died:
                                player.Alive = false;
                                break;
                        }
                    });
                }
                else
                {
                    if (e.Action == PlayerAction.Joined) Dispatcher.Invoke(() => { context.Players.Add(new Player(e.Name, e.Color, !e.IsDead, 0, 0, 0)); });
                }
            }
            Logger.Debug("{@e}", e);
        }




        private void OnJoinedLobby(object sender, LobbyEventArgs e)
        {
            context.GameCode = e.LobbyCode;
            context.GameMap = e.Map;
            this.BeginInvoke(a =>
            {
                if (context.Settings.AlwaysCopyGameCode) Clipboard.SetText(e.LobbyCode);
            });
        }

        private Color PlayerColorToColorOBJ(PlayerColor pColor)
        {
            var OutputCode = Color.White;
            switch (pColor)
            {
                case PlayerColor.Red:
                    OutputCode = Color.Red;
                    break;
                case PlayerColor.Blue:
                    OutputCode = Color.RoyalBlue;
                    break;
                case PlayerColor.Green:
                    OutputCode = Color.Green;
                    break;
                case PlayerColor.Pink:
                    OutputCode = Color.Magenta;
                    break;
                case PlayerColor.Orange:
                    OutputCode = Color.Orange;
                    break;
                case PlayerColor.Yellow:
                    OutputCode = Color.Yellow;
                    break;
                case PlayerColor.Black:
                    OutputCode = Color.Gray;
                    break;
                case PlayerColor.White:
                    OutputCode = Color.White;
                    break;
                case PlayerColor.Purple:
                    OutputCode = Color.MediumPurple;
                    break;
                case PlayerColor.Brown:
                    OutputCode = Color.SaddleBrown;
                    break;
                case PlayerColor.Cyan:
                    OutputCode = Color.Cyan;
                    break;
                case PlayerColor.Lime:
                    OutputCode = Color.Lime;
                    break;
                case PlayerColor.Maroon:
                    OutputCode = Color.Maroon;
                    break;
                case PlayerColor.Rose:
                    OutputCode = Color.MistyRose;
                    break;
                case PlayerColor.Banana:
                    OutputCode = Color.LemonChiffon;
                    break;
                case PlayerColor.Gray:
                    OutputCode = Color.Gray;
                    break;
                case PlayerColor.Tan:
                    OutputCode = Color.Tan;
                    break;
                case PlayerColor.Coral:
                    OutputCode = Color.LightCoral;
                    break;
            }

            return OutputCode;
        }

        private void SetDefaultThemeColor()
        {
            ThemeManager.Current.ThemeSyncMode = ThemeSyncMode.DoNotSync;

            string BaseColor = ThemeManager.BaseColorDark;

            var newTheme2 = new Theme("CustomTheme",
                "CustomTheme",
                BaseColor,
                "CustomAccent",
                System.Windows.Media.Color.FromArgb(255, 140, 158, 255),
                new SolidColorBrush(System.Windows.Media.Color.FromArgb(255, 140, 158, 255)),
                true,
                false);
            ThemeManager.Current.ChangeTheme(this, newTheme2);
        }

        public static void OpenBrowser(string url)
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

        private void ApplyDarkMode()
        {
            if (config.DarkMode)
            {
                context.BackgroundBrush = new ImageBrush(new BitmapImage(new Uri("pack://application:,,,/Resources/Misc/AutoBG.png")));
                ThemeManager.Current.ChangeThemeBaseColor(this, ThemeManager.BaseColorDark);
                NormalTextColor = Color.White;
            }
            else
            {
                context.BackgroundBrush = new ImageBrush(new BitmapImage(new Uri("pack://application:,,,/Resources/Misc/AutoBG.png")));
                ThemeManager.Current.ChangeThemeBaseColor(this, ThemeManager.BaseColorDark);
                NormalTextColor = Color.White;
            }
        }

        private void Settings(object sender, RoutedEventArgs e)
        {
            // Open up the settings flyout
            //Cracked();
            SettingsFlyout.IsOpen = true;
        }

        private void Darkmode_Toggled(object sender, RoutedEventArgs e)
        {
            if (!(sender is ToggleSwitch toggleSwitch)) return;

            ApplyDarkMode();
        }

        private void ManualConnect_Click(object sender, RoutedEventArgs e)
        {
            //Open up the manual connection flyout.
            ManualConnectionFlyout.IsOpen = true;
        }

        private void GameStateChangedHandler(object sender, GameStateChangedEventArgs e)
        {
            setCurrentState(e.NewState);
            while (DeadMessages.Count > 0)
            {
                var playerToKill = DeadMessages.Dequeue();
                if (context.Players.Contains(playerToKill)) playerToKill.Alive = false;
            }
            Logger.Info("State change: {@e}", e);
            if (e.NewState == GameState.MENU)
            {
                setGameCode("");
                Dispatcher.Invoke(() =>
                {
                    context.GameState = e.NewState;
                    foreach (var player in context.Players) player.Alive = true;
                });
            }
            else if (e.NewState == GameState.LOBBY)
            {
                Dispatcher.Invoke(() =>
                {
                    context.GameState = e.NewState;
                    foreach (var player in context.Players) player.Alive = true;
                });
            }

            //Program.conInterface.WriteModuleTextColored("GameMemReader", Color.Green, "State changed to " + e.NewState);
        }

        public void setGameCode(string gamecode)
        {
            context.GameCode = gamecode;
        }

        public void setCurrentState(GameState state)
        {
            context.GameState = state;
        }

        private void RandomizePlayers()
        {
            var dispatcherTimer = new DispatcherTimer();
            dispatcherTimer.Tick += dispatcherTimer_Tick;
            dispatcherTimer.Interval = new TimeSpan(0, 0, 0, 0, 100);
            dispatcherTimer.Start();
        }

        private void dispatcherTimer_Tick(object sender, EventArgs e)
        {
            var r = new Random();
            var playerToChange = context.Players[r.Next(context.Players.Count)];
            var hatID = r.Next(94);
            var Alive = r.Next(0, 2) == 1;
            var pantId = r.Next(0, 16);
            var petID = r.Next(0, 12);
            playerToChange.Alive = Alive;
            if (!Alive)
            {
                playerToChange.HatID = (uint)hatID;
                playerToChange.PantsID = (uint)pantId;
                playerToChange.PetID = (uint)petID;
            }



        }

        private void TestUsers()
        {
            context.Connected = true;
            context.GameState = GameState.TASKS;
            var numOfPlayers = 14;
            for (uint i = 0; i < numOfPlayers; i++) context.Players.Add(new Player($"{i}Cool4u", (PlayerColor)(i % 12), true, i % 10, i, 0));

            RandomizePlayers();
        }


        private void MetroWindow_Loaded(object sender, RoutedEventArgs e)
        {

            //TestUsers();
        }

        public void PlayGotEm()
        {
            this.BeginInvoke(win =>
            {
                //win.MemeFlyout.IsOpen = true;
                //win.MemePlayer.Position = TimeSpan.Zero;
            });
        }

        private async void MainWindow_OnContentRendered(object? sender, EventArgs e)
        {
            //TestFillConsole(10);
            //setCurrentState("GAMESTATE");
            //setGameCode("GAMECODE");
            SetDefaultThemeColor();

            ApplyDarkMode();
            if (!config.startupMemes)
            {
                Logger.Info("Meme Module disabled :(");
            }

            await StartLocalBotOrOfferSetupAsync();
        }

        private async void SubmitConnectButton_OnClick(object sender, RoutedEventArgs e)
        {
            await PairAsync(Host.Text, Code.Text);
        }

        private void MemePlayer_OnMediaEnded(object sender, RoutedEventArgs e)
        {
            this.BeginInvoke(win =>
            {
                //win.MemeFlyout.IsOpen = false;
            });
        }

        //private void MemeFlyout_OnIsOpenChanged(object sender, RoutedEventArgs e)
        //{
        //if (MemeFlyout.IsOpen)
        //{
        //    MemePlayer.Play();
        //    Task.Factory.StartNew(() =>
        //   {
        //       Thread.Sleep(5000);
        //        MemeFlyout.Invoke(new Action(() =>
        //        {
        //            if (MemeFlyout.IsOpen)
        //             {
        //                 MemeFlyout.CloseButtonVisibility = Visibility.Visible;
        //             }
        //        }));
        //
        //      });
        // }
        // else
        // {
        //    MemeFlyout.CloseButtonVisibility = Visibility.Hidden;
        //    MemePlayer.Close();
        //    GC.Collect();
        // }
        //}
        private async void ReloadOffsetsButton_OnClick(object sender, RoutedEventArgs e)
        {
            GameMemReader.getInstance().offMan.refreshLocal();
            await GameMemReader.getInstance().offMan.RefreshIndex();
            GameMemReader.getInstance().CurrentOffsets = GameMemReader.getInstance().offMan
                .FetchForHash(GameMemReader.getInstance().GameHash);
            if (GameMemReader.getInstance().CurrentOffsets is not null)
            {
                //WriteConsoleLineFormatted("GameMemReader", Color.Lime, $"Loaded offsets: {GameMemReader.getInstance().CurrentOffsets.Description}");
            }
        }

        private void APIServerToggleSwitch_Toggled(object sender, RoutedEventArgs e)
        {
            if (!(sender is ToggleSwitch toggleSwitch)) return;

            if (config.ApiServer)
            {
                Logger.Info("API server starting");
                ServerSocket.instance.Start();
            }
            else
            {
                Logger.Info("API server stopping");
                ServerSocket.instance.Stop();
            }
        }

        private async void ResetConfigButton_OnClick(object sender, RoutedEventArgs e)
        {
            var result = await this.ShowMessageAsync(Properties.Resources.ResetConfigQuestionTitle,
                Properties.Resources.ResetConfigQuestion,
                MessageDialogStyle.AffirmativeAndNegative, new MetroDialogSettings { AnimateShow = true, AnimateHide = false });
            if (result == MessageDialogResult.Affirmative)
            {
                var progressBar = await context.DialogCoordinator.ShowProgressAsync(context, Properties.Resources.ResettingConfigTitle,
                    Properties.Resources.PleaseWait, false, new MetroDialogSettings { AnimateHide = false, AnimateShow = false });
                progressBar.Minimum = 0;
                progressBar.Maximum = 1;
                if (File.Exists(Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData),
                    "\\AmongUsCapture\\AmongUsGUI", "Settings.json")))
                    File.Delete(Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "\\AmongUsCapture\\AmongUsGUI", "Settings.json"));

                if (File.Exists(Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData),
                    "AmongUsCapture", "Settings.json")))
                    File.Delete(Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "AmongUsCapture", "Settings.json"));

                for (var i = 0; i < 100; i++) //Useless loading to make the user think we are doing a big task
                {
                    var currentPercent = i / 100d;
                    progressBar.SetProgress(currentPercent);
                    await Task.Delay(10);
                }

                await progressBar.CloseAsync();
                var selection = await this.ShowMessageAsync(Properties.Resources.ConfigResetTitle,
                    Properties.Resources.ConfigResetMessage,
                    MessageDialogStyle.AffirmativeAndNegative, new MetroDialogSettings { AnimateHide = true, AffirmativeButtonText = Properties.Resources.RestartText, NegativeButtonText = Properties.Resources.ExitText });
                if (selection == MessageDialogResult.Affirmative)
                {
                    IPCAdapter.getInstance().mutex.ReleaseMutex(); //Release the mutex so the other app does not see us. 
                    Process.Start(Process.GetCurrentProcess().MainModule.FileName);
                    Application.Current.Shutdown(0);
                }
                else
                {
                    Application.Current.Shutdown(0);
                }
            }
        }

        private void AlwaysOnTopSwitch_OnToggled(object sender, RoutedEventArgs e)
        {
            Window.Topmost = context.Settings.alwaysOnTop;
        }

        private void ContributorsButton_OnClick(object sender, RoutedEventArgs e)
        {
            Contributors c = new Contributors(context.Settings.DarkMode);
            c.Show();
        }

        private void PremiumButton_OnClick(object sender, RoutedEventArgs e)
        {
            OpenBrowser("https://automute.us/premium");
        }

        private void OpenLogsFolderButton_OnClick(object sender, RoutedEventArgs e)
        {
            if (!Directory.Exists(App.LogFolder)) return;
            Process.Start(new ProcessStartInfo(App.LogFolder) { UseShellExecute = true });
        }
    }
}