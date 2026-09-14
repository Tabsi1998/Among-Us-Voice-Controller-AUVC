using System;
using System.Collections.Generic;
using System.Linq;
using System.Net.Http;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Threading;
using AUVC.Transport;
using ControlzEx.Theming;
using MahApps.Metro.Controls;

namespace AUCapture_WPF
{
    /// <summary>
    /// Sets up the bot on this PC: the token, the invite, the server, the
    /// channels, and capture's connection to it.
    /// </summary>
    public partial class SetupWindow
    {
        private const int TokenStep = 0;
        private const int ServerStep = 1;
        private const int ChannelStep = 2;
        private const int DoneStep = 3;

        private readonly IAppSettings settings;
        private readonly DispatcherTimer serverRefresh = new() { Interval = TimeSpan.FromSeconds(3) };

        private int step;
        private bool busy;
        private BotApplication application;
        private string guildId = "";

        public SetupWindow(IAppSettings settings)
        {
            InitializeComponent();
            this.settings = settings;

            ApplyTexts();
            serverRefresh.Tick += async (_, _) => await RefreshServersAsync();
            Loaded += (_, _) =>
            {
                var theme = ThemeManager.Current.DetectTheme(Application.Current.MainWindow);
                if (theme is not null)
                {
                    ThemeManager.Current.ChangeTheme(this, theme);
                }
            };
            Closed += OnClosed;

            if (LocalBot.HasToken)
            {
                TokenResult.Text = SetupText.TokenStored;
            }
            ShowStep(TokenStep);
        }

        /// <summary>Whether the setup was finished rather than abandoned.</summary>
        public bool Completed { get; private set; }

        private void ApplyTexts()
        {
            Title = SetupText.WindowTitle;
            TokenIntro.Text = SetupText.TokenIntro;
            OpenPortalButton.Content = SetupText.OpenPortal;
            TextBoxHelper.SetWatermark(TokenBox, SetupText.TokenPlaceholder);
            CheckTokenButton.Content = SetupText.CheckToken;

            ServerIntro.Text = SetupText.ServerIntro;
            InviteButton.Content = SetupText.Invite;
            RefreshServersButton.Content = SetupText.Refresh;

            ChannelIntro.Text = SetupText.ChannelIntro;
            MainChannelLabel.Text = SetupText.MainChannel;
            GhostChannelLabel.Text = SetupText.GhostChannel;
            ControlChannelLabel.Text = SetupText.ControlChannel;
            AutoStartBox.Content = SetupText.AutoStart;
            RefreshChannelsButton.Content = SetupText.Refresh;

            DoneIntro.Text = SetupText.DoneIntro;
            ChecksHeading.Text = SetupText.ChecksHeading;
            RefreshChecksButton.Content = SetupText.Refresh;

            CancelButton.Content = SetupText.Cancel;
            BackButton.Content = SetupText.Back;
        }

        private void ShowStep(int value)
        {
            step = value;

            TokenPanel.Visibility = Visible(step == TokenStep);
            ServerPanel.Visibility = Visible(step == ServerStep);
            ChannelPanel.Visibility = Visible(step == ChannelStep);
            DonePanel.Visibility = Visible(step == DoneStep);

            StepTitle.Text = step switch
            {
                TokenStep => SetupText.TokenTitle,
                ServerStep => SetupText.ServerTitle,
                ChannelStep => SetupText.ChannelTitle,
                _ => SetupText.DoneTitle,
            };

            BackButton.Visibility = Visible(step is ServerStep or ChannelStep);
            CancelButton.Visibility = Visible(step != DoneStep);
            NextButton.Content = step == DoneStep ? SetupText.Finish : SetupText.Next;

            if (step == ServerStep)
            {
                serverRefresh.Start();
            }
            else
            {
                serverRefresh.Stop();
            }
            UpdateButtons();
        }

        private static Visibility Visible(bool visible) => visible ? Visibility.Visible : Visibility.Collapsed;

        private void SetBusy(bool value)
        {
            busy = value;
            UpdateButtons();
        }

        private void UpdateButtons()
        {
            BackButton.IsEnabled = !busy;
            NextButton.IsEnabled = !busy && step switch
            {
                TokenStep => application is not null || LocalBot.HasToken,
                ServerStep => LocalBot.IsRunning && ServerList.SelectedItem is LocalServer,
                _ => true,
            };
        }

        private async void NextButton_Click(object sender, RoutedEventArgs e)
        {
            switch (step)
            {
                case TokenStep:
                    ShowStep(ServerStep);
                    await StartBotAsync();
                    break;

                case ServerStep:
                    guildId = ((LocalServer)ServerList.SelectedItem).Id;
                    ShowStep(ChannelStep);
                    await LoadChannelsAsync();
                    break;

                case ChannelStep:
                    await SaveChannelsAsync();
                    break;

                default:
                    Completed = true;
                    Close();
                    break;
            }
        }

        private void BackButton_Click(object sender, RoutedEventArgs e) => ShowStep(Math.Max(TokenStep, step - 1));

        private void CancelButton_Click(object sender, RoutedEventArgs e) => Close();

        // Step 1: the token.

        private void OpenPortalButton_Click(object sender, RoutedEventArgs e) =>
            MainWindow.OpenBrowser("https://discord.com/developers/applications");

        private async void CheckTokenButton_Click(object sender, RoutedEventArgs e)
        {
            SetBusy(true);
            CheckTokenButton.IsEnabled = false;
            TokenResult.Text = SetupText.Checking;
            try
            {
                var pasted = TokenBox.Password;
                application = await LocalBot.Discord.CheckTokenAsync(pasted);

                // A new token can mean a different bot, so one already running
                // from an earlier setup has to make way.
                await LocalBot.StopAsync();
                LocalBot.SaveToken(pasted);
                TokenBox.Clear();

                TokenResult.Text = SetupText.TokenOk(application.BotName);
                if (application.RequiresCodeGrant)
                {
                    TokenResult.Text += Environment.NewLine + Environment.NewLine + SetupText.CodeGrantWarning;
                }
            }
            catch (BotTokenException error)
            {
                application = null;
                TokenResult.Text = SetupText.TokenProblemText(error.Problem);
            }
            finally
            {
                CheckTokenButton.IsEnabled = true;
                SetBusy(false);
            }
        }

        // Step 2: invite and server.

        private async Task StartBotAsync()
        {
            SetBusy(true);
            BotState.Text = SetupText.StartingBot;
            try
            {
                application ??= await LocalBot.CheckStoredTokenAsync();
                var status = await LocalBot.StartAsync();
                BotState.Text = SetupText.BotOnline(status.BotName);
                ShowServers(status.Servers);
            }
            catch (BotTokenException error)
            {
                BotState.Text = SetupText.TokenProblemText(error.Problem);
            }
            catch (BotHostException error)
            {
                BotState.Text = SetupText.BotStartFailed(error);
            }
            catch (Exception error) when (error is HttpRequestException or LocalControlException or TaskCanceledException)
            {
                BotState.Text = "✖ " + error.Message;
            }
            finally
            {
                SetBusy(false);
            }
        }

        private void InviteButton_Click(object sender, RoutedEventArgs e)
        {
            if (application is not null)
            {
                MainWindow.OpenBrowser(DiscordApplicationClient.InviteUrl(application.ApplicationId).ToString());
            }
        }

        private async void RefreshServersButton_Click(object sender, RoutedEventArgs e)
        {
            if (LocalBot.IsRunning)
            {
                await RefreshServersAsync();
            }
            else
            {
                await StartBotAsync();
            }
        }

        private async Task RefreshServersAsync()
        {
            var control = LocalBot.Control;
            if (busy || control is null)
            {
                return;
            }

            try
            {
                var status = await control.GetStatusAsync();
                BotState.Text = SetupText.BotOnline(status.BotName);
                ShowServers(status.Servers);
            }
            catch (Exception error) when (error is HttpRequestException or LocalControlException or TaskCanceledException)
            {
                // The next refresh tries again.
            }
        }

        private void ShowServers(IReadOnlyList<LocalServer> servers)
        {
            var current = ServerList.ItemsSource as IReadOnlyList<LocalServer>;
            if (current is null || !current.Select(server => server.Id).SequenceEqual(servers.Select(server => server.Id)))
            {
                var selectedId = (ServerList.SelectedItem as LocalServer)?.Id ?? settings.botGuildId;
                ServerList.ItemsSource = servers;
                ServerList.SelectedItem = servers.FirstOrDefault(server => server.Id == selectedId)
                                          ?? (servers.Count == 1 ? servers[0] : null);
            }

            if (servers.Count == 0)
            {
                BotState.Text += Environment.NewLine + SetupText.NoServersYet;
            }
            UpdateButtons();
        }

        private void ServerList_SelectionChanged(object sender, SelectionChangedEventArgs e) => UpdateButtons();

        // Step 3: channels.

        private async void RefreshChannelsButton_Click(object sender, RoutedEventArgs e) => await LoadChannelsAsync();

        private async Task LoadChannelsAsync()
        {
            var control = LocalBot.Control;
            if (control is null)
            {
                ChannelResult.Text = SetupText.BotNotRunning;
                return;
            }

            SetBusy(true);
            ChannelResult.Text = "";
            try
            {
                var channels = await control.GetChannelsAsync(guildId);
                var guild = await control.GetGuildAsync(guildId);

                var voice = channels.Where(channel => channel.Kind == LocalChannel.Voice).ToList();
                var text = new List<LocalChannel> { new() { Id = "", Name = SetupText.NoControlChannel } };
                text.AddRange(channels.Where(channel => channel.Kind == LocalChannel.Text));

                MainChannelBox.ItemsSource = voice;
                GhostChannelBox.ItemsSource = voice;
                ControlChannelBox.ItemsSource = text;

                MainChannelBox.SelectedItem = voice.FirstOrDefault(channel => channel.Id == guild.MainVoiceChannelId)
                                              ?? voice.ElementAtOrDefault(0);
                GhostChannelBox.SelectedItem = voice.FirstOrDefault(channel => channel.Id == guild.GhostVoiceChannelId)
                                               ?? voice.ElementAtOrDefault(1);
                ControlChannelBox.SelectedItem = text.FirstOrDefault(channel => channel.Id == guild.ControlTextChannelId)
                                                 ?? text[0];
                AutoStartBox.IsChecked = true;

                if (voice.Count < 2)
                {
                    ChannelResult.Text = SetupText.TooFewVoiceChannels;
                }
            }
            catch (Exception error) when (error is HttpRequestException or LocalControlException or TaskCanceledException)
            {
                ChannelResult.Text = "✖ " + error.Message;
            }
            finally
            {
                SetBusy(false);
            }
        }

        private async Task SaveChannelsAsync()
        {
            if (MainChannelBox.SelectedItem is not LocalChannel main ||
                GhostChannelBox.SelectedItem is not LocalChannel ghost ||
                main.Id == ghost.Id)
            {
                ChannelResult.Text = SetupText.NeedTwoVoiceChannels;
                return;
            }

            var control = LocalBot.Control;
            if (control is null)
            {
                ChannelResult.Text = SetupText.BotNotRunning;
                return;
            }

            SetBusy(true);
            ChannelResult.Text = SetupText.Saving;
            try
            {
                await control.SetupAsync(guildId, new LocalSetup
                {
                    MainVoiceChannelId = main.Id,
                    GhostVoiceChannelId = ghost.Id,
                    ControlTextChannelId = (ControlChannelBox.SelectedItem as LocalChannel)?.Id ?? "",
                    AutoStart = AutoStartBox.IsChecked == true,
                });

                settings.runBotOnThisPc = true;
                settings.botGuildId = guildId;
                await LocalBot.ConnectCaptureAsync(guildId, freshCredential: true);

                ChannelResult.Text = "";
                ShowStep(DoneStep);
                await RefreshChecksAsync();
            }
            catch (Exception error) when (error is HttpRequestException or LocalControlException
                                              or InvalidOperationException or TaskCanceledException)
            {
                ChannelResult.Text = "✖ " + error.Message;
            }
            finally
            {
                SetBusy(false);
            }
        }

        // Step 4: done.

        private async void RefreshChecksButton_Click(object sender, RoutedEventArgs e) => await RefreshChecksAsync();

        private async Task RefreshChecksAsync()
        {
            var control = LocalBot.Control;
            if (control is null)
            {
                ChecksList.ItemsSource = new[] { SetupText.BotNotRunning };
                return;
            }

            try
            {
                var guild = await control.GetGuildAsync(guildId);
                ChecksList.ItemsSource = guild.Checks.Select(Describe).ToList();
            }
            catch (Exception error) when (error is HttpRequestException or LocalControlException or TaskCanceledException)
            {
                ChecksList.ItemsSource = new[] { "✖ " + error.Message };
            }
        }

        /// <summary>One doctor check as a line of text. The doctor writes for Discord, so its markdown goes.</summary>
        internal static string Describe(LocalCheck check)
        {
            var icon = check.Level switch
            {
                LocalCheck.Fail => "❌",
                LocalCheck.Warn => "⚠️",
                _ => "✅",
            };
            var line = $"{icon} {check.Name}: {Plain(check.Detail)}";
            return string.IsNullOrEmpty(check.Fix) ? line : line + " → " + Plain(check.Fix);
        }

        private static string Plain(string markdown) => markdown.Replace("**", "").Replace("`", "");

        private void OnClosed(object sender, EventArgs e)
        {
            serverRefresh.Stop();

            // A bot started only for a setup that was then abandoned has nothing
            // to do, and nobody would know it is running.
            if (!Completed && !settings.runBotOnThisPc)
            {
                _ = LocalBot.StopAsync();
            }
        }
    }
}
