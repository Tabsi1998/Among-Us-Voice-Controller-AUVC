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
using MahApps.Metro.Controls.Dialogs;

namespace AUCapture_WPF
{
    /// <summary>
    /// Sets up the bot on this PC, and changes that setup later.
    /// </summary>
    /// <remarks>
    /// The first run walks through four steps in order: the token, the invite and
    /// server, the channels, and the result. Opened again once the bot is set up,
    /// the same four parts become sections that can be visited in any order and
    /// changed one at a time, so nothing needs reinstalling or setting up again
    /// from the start.
    /// </remarks>
    public partial class SetupWindow
    {
        private const int TokenStep = 0;
        private const int ServerStep = 1;
        private const int ChannelStep = 2;
        private const int DoneStep = 3;
        private const int PlayersStep = 4;

        private readonly IAppSettings settings;
        private readonly bool editing;
        private readonly DispatcherTimer serverRefresh = new() { Interval = TimeSpan.FromSeconds(3) };

        private int step;
        private bool busy;
        private BotApplication application;
        private string guildId;

        /// <param name="editing">
        /// Open the settings of a bot that is already set up, rather than the
        /// first-run steps.
        /// </param>
        public SetupWindow(IAppSettings settings, bool editing = false)
        {
            InitializeComponent();
            this.settings = settings;
            this.editing = editing;
            guildId = settings.botGuildId ?? "";

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

            if (editing)
            {
                SectionList.ItemsSource = new[]
                {
                    SetupText.SectionToken, SetupText.SectionServer, SetupText.SectionChannels, SetupText.SectionStatus,
                    SetupText.SectionPlayers,
                };
                SectionList.Visibility = Visibility.Visible;
                UseServerButton.Visibility = Visibility.Visible;
                SaveChannelsButton.Visibility = Visibility.Visible;
                RestartBotButton.Visibility = Visibility.Visible;
                DisableBotButton.Visibility = Visibility.Visible;
                SectionList.SelectedIndex = DoneStep;
            }
            else
            {
                ShowStep(TokenStep);
            }
        }

        /// <summary>Whether the window was finished or closed on purpose rather than abandoned.</summary>
        public bool Completed { get; private set; }

        private void ApplyTexts()
        {
            Title = editing ? SetupText.SettingsWindowTitle : SetupText.WindowTitle;
            TokenIntro.Text = SetupText.TokenIntro;
            OpenPortalButton.Content = SetupText.OpenPortal;
            TextBoxHelper.SetWatermark(TokenBox, SetupText.TokenPlaceholder);
            CheckTokenButton.Content = SetupText.CheckToken;

            ServerIntro.Text = SetupText.ServerIntro;
            InviteButton.Content = SetupText.Invite;
            RefreshServersButton.Content = SetupText.Refresh;
            UseServerButton.Content = SetupText.UseServer;

            ChannelIntro.Text = SetupText.ChannelIntro;
            MainChannelLabel.Text = SetupText.MainChannel;
            GhostChannelLabel.Text = SetupText.GhostChannel;
            ControlChannelLabel.Text = SetupText.ControlChannel;
            AutoStartBox.Content = SetupText.AutoStart;
            SaveChannelsButton.Content = SetupText.Save;
            RefreshChannelsButton.Content = SetupText.Refresh;

            DoneIntro.Text = editing ? SetupText.StatusIntro : SetupText.DoneIntro;
            ChecksHeading.Text = SetupText.ChecksHeading;
            RefreshChecksButton.Content = SetupText.Refresh;
            RestartBotButton.Content = SetupText.RestartBot;
            DisableBotButton.Content = SetupText.DisableBot;

            PlayersIntro.Text = SetupText.PlayersIntro;
            RefreshPlayersButton.Content = SetupText.Refresh;

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
            PlayersPanel.Visibility = Visible(step == PlayersStep);

            StepTitle.Text = editing
                ? SetupText.SettingsTitle(step)
                : step switch
                {
                    TokenStep => SetupText.TokenTitle,
                    ServerStep => SetupText.ServerTitle,
                    ChannelStep => SetupText.ChannelTitle,
                    _ => SetupText.DoneTitle,
                };

            BackButton.Visibility = Visible(!editing && step is ServerStep or ChannelStep);
            CancelButton.Visibility = Visible(!editing && step != DoneStep);
            NextButton.Content = editing ? SetupText.Close : step == DoneStep ? SetupText.Finish : SetupText.Next;

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
            SectionList.IsEnabled = !busy;
            UseServerButton.IsEnabled = !busy && LocalBot.IsRunning && ServerList.SelectedItem is LocalServer;
            SaveChannelsButton.IsEnabled = !busy;
            RestartBotButton.IsEnabled = !busy;
            DisableBotButton.IsEnabled = !busy && settings.runBotOnThisPc;

            NextButton.IsEnabled = !busy && (editing || step switch
            {
                TokenStep => application is not null || LocalBot.HasToken,
                ServerStep => LocalBot.IsRunning && ServerList.SelectedItem is LocalServer,
                _ => true,
            });
        }

        private async void NextButton_Click(object sender, RoutedEventArgs e)
        {
            if (editing)
            {
                Completed = true;
                Close();
                return;
            }

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
                    await SaveChannelsAsync(finishSetup: true);
                    break;

                default:
                    Completed = true;
                    Close();
                    break;
            }
        }

        private void BackButton_Click(object sender, RoutedEventArgs e) => ShowStep(Math.Max(TokenStep, step - 1));

        private void CancelButton_Click(object sender, RoutedEventArgs e) => Close();

        private async void SectionList_SelectionChanged(object sender, SelectionChangedEventArgs e)
        {
            if (!editing || SectionList.SelectedIndex < 0)
            {
                return;
            }

            ShowStep(SectionList.SelectedIndex);
            switch (step)
            {
                case ServerStep:
                    if (LocalBot.IsRunning)
                    {
                        await RefreshServersAsync();
                    }
                    else
                    {
                        await StartBotAsync();
                    }
                    break;

                case ChannelStep:
                    if (string.IsNullOrEmpty(guildId))
                    {
                        ChannelResult.Text = SetupText.ChooseServerFirst;
                    }
                    else if (await EnsureBotRunningAsync())
                    {
                        await LoadChannelsAsync();
                    }
                    break;

                case DoneStep:
                    await RefreshChecksAsync();
                    break;

                case PlayersStep:
                    if (string.IsNullOrEmpty(guildId))
                    {
                        PlayersResult.Text = SetupText.ChooseServerFirst;
                    }
                    else if (await EnsureBotRunningAsync())
                    {
                        await LoadPlayersAsync();
                    }
                    break;
            }
        }

        // The token.

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

                // A new token can mean a different bot, so one already running has
                // to make way for it.
                var wasRunning = LocalBot.IsRunning;
                await LocalBot.StopAsync();
                LocalBot.SaveToken(pasted);
                TokenBox.Clear();

                TokenResult.Text = SetupText.TokenOk(application.BotName);
                if (application.RequiresCodeGrant)
                {
                    TokenResult.Text += Environment.NewLine + Environment.NewLine + SetupText.CodeGrantWarning;
                }

                if (editing && (wasRunning || settings.runBotOnThisPc))
                {
                    TokenResult.Text += Environment.NewLine + Environment.NewLine + SetupText.TokenChangedRestart;
                    await RestartBotAsync(TokenResult);
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

        // The invite and the server.

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

        private async Task<bool> EnsureBotRunningAsync()
        {
            if (LocalBot.IsRunning)
            {
                return true;
            }

            await StartBotAsync();
            if (!LocalBot.IsRunning)
            {
                ChannelResult.Text = BotState.Text;
                StatusText.Text = BotState.Text;
                PlayersResult.Text = BotState.Text;
            }
            return LocalBot.IsRunning;
        }

        private async void InviteButton_Click(object sender, RoutedEventArgs e)
        {
            try
            {
                application ??= await LocalBot.CheckStoredTokenAsync();
                MainWindow.OpenBrowser(DiscordApplicationClient.InviteUrl(application.ApplicationId).ToString());
            }
            catch (BotTokenException error)
            {
                BotState.Text = SetupText.TokenProblemText(error.Problem);
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
                var selectedId = (ServerList.SelectedItem as LocalServer)?.Id ?? guildId;
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

        private async void UseServerButton_Click(object sender, RoutedEventArgs e)
        {
            if (ServerList.SelectedItem is not LocalServer server)
            {
                return;
            }

            SetBusy(true);
            try
            {
                // A credential belongs to one server, so capture gets a new one.
                await LocalBot.ConnectCaptureAsync(server.Id, freshCredential: true);
                guildId = server.Id;
                settings.botGuildId = server.Id;
                BotState.Text = SetupText.ServerSaved(server.Name);
            }
            catch (Exception error) when (error is HttpRequestException or LocalControlException
                                              or InvalidOperationException or TaskCanceledException)
            {
                BotState.Text = "✖ " + error.Message;
            }
            finally
            {
                SetBusy(false);
            }
        }

        // The channels.

        private async void RefreshChannelsButton_Click(object sender, RoutedEventArgs e) => await LoadChannelsAsync();

        private async void SaveChannelsButton_Click(object sender, RoutedEventArgs e) =>
            await SaveChannelsAsync(finishSetup: false);

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

                // A first setup turns automatic start on; changing the channels
                // later keeps whatever was chosen.
                AutoStartBox.IsChecked = !editing || guild.AutoStart;

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

        private async Task SaveChannelsAsync(bool finishSetup)
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

                if (!finishSetup)
                {
                    ChannelResult.Text = SetupText.Saved;
                    return;
                }

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

        // The players.

        private async void RefreshPlayersButton_Click(object sender, RoutedEventArgs e) => await LoadPlayersAsync();

        private async Task LoadPlayersAsync()
        {
            var control = LocalBot.Control;
            if (control is null)
            {
                CrewmateList.ItemsSource = null;
                PlayersResult.Text = SetupText.BotNotRunning;
                return;
            }
            if (string.IsNullOrEmpty(guildId))
            {
                PlayersResult.Text = SetupText.ChooseServerFirst;
                return;
            }

            SetBusy(true);
            PlayersResult.Text = "";
            try
            {
                ShowCrewmates(await control.GetCrewmatesAsync(guildId));
            }
            catch (Exception error) when (error is HttpRequestException or LocalControlException or TaskCanceledException)
            {
                PlayersResult.Text = "✖ " + error.Message;
            }
            finally
            {
                SetBusy(false);
            }
        }

        private void ShowCrewmates(LocalCrewmates crewmates)
        {
            CrewmateList.ItemsSource = CrewmateRow.From(crewmates);
            if (crewmates.Players.Count == 0)
            {
                PlayersResult.Text = SetupText.NoLobbyYet;
            }
            else if (crewmates.Members.Count == 0)
            {
                PlayersResult.Text = SetupText.NobodyInVoice;
            }
        }

        /// <summary>
        /// Links the crewmate of a row to the member just chosen for it.
        /// </summary>
        /// <remarks>
        /// A row's menu also reports a selection when it first shows the member the
        /// bot already has, which must not be sent back as a change.
        /// </remarks>
        private async void CrewmateMember_SelectionChanged(object sender, SelectionChangedEventArgs e)
        {
            if (sender is not ComboBox { DataContext: CrewmateRow row } ||
                e.AddedItems.Count != 1 || e.AddedItems[0] is not LocalMember chosen ||
                chosen.Id == row.Selected?.Id)
            {
                return;
            }

            var control = LocalBot.Control;
            if (control is null || string.IsNullOrEmpty(guildId))
            {
                PlayersResult.Text = control is null ? SetupText.BotNotRunning : SetupText.ChooseServerFirst;
                return;
            }

            SetBusy(true);
            PlayersResult.Text = SetupText.Saving;
            try
            {
                var crewmates = await control.LinkAsync(guildId, row.Player, chosen.Id);
                row.Selected = chosen;
                ShowCrewmates(crewmates);
                PlayersResult.Text = string.IsNullOrEmpty(chosen.Id)
                    ? SetupText.CrewmateUnlinked(row.Player)
                    : SetupText.CrewmateLinked(row.Player, chosen.Name);
            }
            catch (Exception error) when (error is HttpRequestException or LocalControlException or TaskCanceledException)
            {
                SetBusy(false);
                // Show the lobby as the bot has it again, so the menu does not
                // pretend the change was made.
                await LoadPlayersAsync();
                PlayersResult.Text = "✖ " + error.Message;
            }
            finally
            {
                SetBusy(false);
            }
        }

        // The result and the status.

        private async void RefreshChecksButton_Click(object sender, RoutedEventArgs e) => await RefreshChecksAsync();

        private async void RestartBotButton_Click(object sender, RoutedEventArgs e)
        {
            SetBusy(true);
            try
            {
                await RestartBotAsync(StatusText);
                await RefreshChecksAsync();
            }
            finally
            {
                SetBusy(false);
            }
        }

        private async void DisableBotButton_Click(object sender, RoutedEventArgs e)
        {
            var answer = await this.ShowMessageAsync(SetupText.DisableBot, SetupText.DisableBotQuestion,
                MessageDialogStyle.AffirmativeAndNegative,
                new MetroDialogSettings { AffirmativeButtonText = SetupText.DisableBot, NegativeButtonText = SetupText.Cancel });
            if (answer != MessageDialogResult.Affirmative)
            {
                return;
            }

            SetBusy(true);
            try
            {
                settings.runBotOnThisPc = false;
                await LocalBot.StopAsync();
                StatusText.Text = SetupText.BotDisabled;
                ChecksList.ItemsSource = null;
            }
            finally
            {
                SetBusy(false);
            }
        }

        /// <summary>
        /// Stops and starts the bot, and points capture at it again, reporting into
        /// the given text.
        /// </summary>
        private async Task RestartBotAsync(TextBlock report)
        {
            var before = report.Text;
            report.Text = before + Environment.NewLine + SetupText.Restarting;
            try
            {
                await LocalBot.StopAsync();
                var status = await LocalBot.StartAsync();
                if (!string.IsNullOrEmpty(guildId))
                {
                    await LocalBot.ConnectCaptureAsync(guildId, freshCredential: false);
                }
                report.Text = before + Environment.NewLine + SetupText.BotOnline(status.BotName);
            }
            catch (BotHostException error)
            {
                report.Text = before + Environment.NewLine + SetupText.BotStartFailed(error);
            }
            catch (Exception error) when (error is BotTokenException or HttpRequestException or LocalControlException
                                              or InvalidOperationException or TaskCanceledException)
            {
                report.Text = before + Environment.NewLine + "✖ " + error.Message;
            }
        }

        private async Task RefreshChecksAsync()
        {
            if (editing)
            {
                StatusText.Text = !settings.runBotOnThisPc
                    ? SetupText.BotDisabled
                    : LocalBot.IsRunning ? SetupText.BotRunning : SetupText.BotStopped;
            }

            var control = LocalBot.Control;
            if (control is null || string.IsNullOrEmpty(guildId))
            {
                ChecksList.ItemsSource = control is null ? new[] { SetupText.BotNotRunning } : new[] { SetupText.ChooseServerFirst };
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

            // A bot started only for a first setup that was then abandoned has
            // nothing to do, and nobody would know it is running.
            if (!editing && !Completed && !settings.runBotOnThisPc)
            {
                _ = LocalBot.StopAsync();
            }
        }
    }
}
