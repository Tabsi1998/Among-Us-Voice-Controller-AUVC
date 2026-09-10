using System;
using System.Collections.Generic;
using System.ComponentModel;
using System.IO;
using System.Net.Http;
using System.Threading.Tasks;
using Newtonsoft.Json;

namespace AUOffsetManager
{
    public class OffsetManager
    {
        public static int GameMemReaderVersion = 1; //GameMemReader should update this.

        // Offset index shipped inside the assembly. AUVC must be able to read the game
        // without contacting a third-party host, so this is the base layer that is
        // always present; cached and remote entries are merged on top of it.
        private const string BundledIndexResource = "AUOffsetManager.Offsets.json";
        private Dictionary<string, GameOffsets> OffsetIndex = new Dictionary<string, GameOffsets>();
        private Dictionary<string, GameOffsets> LocalOffsetIndex = new Dictionary<string, GameOffsets>();
        public string indexURL;
        private string StorageLocation = Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "\\AmongUsCapture\\index.json");
        private string StorageLocationCache = Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "\\AmongUsCapture\\indexCache.json");

        public Task indexTask;
        public OffsetManager(string indexURL = "")
        {
            this.indexURL = indexURL;
            OffsetIndex = LoadBundledIndex();

            if (File.Exists(StorageLocation))
            {
                LocalOffsetIndex = JsonConvert.DeserializeObject<Dictionary<string, GameOffsets>>(File.ReadAllText(StorageLocation));
                if (LocalOffsetIndex is null)
                {
                    LocalOffsetIndex = new Dictionary<string, GameOffsets>();
                }
            }

            indexTask = RefreshIndex();
        }

        /// <summary>
        /// Reads the offset index embedded in this assembly. It is the only source that
        /// cannot be taken away by a network failure or by a third party moving a file.
        /// </summary>
        public static Dictionary<string, GameOffsets> LoadBundledIndex()
        {
            using var stream = typeof(OffsetManager).Assembly.GetManifestResourceStream(BundledIndexResource);
            if (stream is null)
            {
                return new Dictionary<string, GameOffsets>();
            }

            using var reader = new StreamReader(stream);
            return JsonConvert.DeserializeObject<Dictionary<string, GameOffsets>>(reader.ReadToEnd())
                   ?? new Dictionary<string, GameOffsets>();
        }

        private void MergeOver(Dictionary<string, GameOffsets> updates)
        {
            if (updates is null)
            {
                return;
            }

            foreach (var entry in updates)
            {
                OffsetIndex[entry.Key] = entry.Value;
            }
        }
        public async Task RefreshIndex()
        {
            if (string.IsNullOrWhiteSpace(indexURL))
            {
                // No remote configured. The bundled index stands on its own; a cache
                // from an earlier run may still add newer game versions on top.
                MergeOver(ReadCache());
                return;
            }

            try
            {
                using var httpClient = new HttpClient();
                var json = await httpClient.GetStringAsync(indexURL);
                var fetched = JsonConvert.DeserializeObject<Dictionary<string, GameOffsets>>(json);
                MergeOver(fetched);
                WriteCache(fetched);
            }
            catch (Exception e)
            {
                // A failed refresh is not fatal: the bundled index already covers the
                // game versions known at build time. Never fall back to a third-party host.
                Console.WriteLine("Offset index refresh from " + indexURL + " failed, using the bundled index. " + e.Message);
                MergeOver(ReadCache());
            }
        }

        private Dictionary<string, GameOffsets> ReadCache()
        {
            try
            {
                return File.Exists(StorageLocationCache)
                    ? JsonConvert.DeserializeObject<Dictionary<string, GameOffsets>>(File.ReadAllText(StorageLocationCache))
                    : null;
            }
            catch (Exception e)
            {
                Console.WriteLine("Could not read the cached offset index. " + e.Message);
                return null;
            }
        }

        private void WriteCache(Dictionary<string, GameOffsets> index)
        {
            if (index is null)
            {
                return;
            }

            try
            {
                var directory = Path.GetDirectoryName(StorageLocationCache);
                if (!string.IsNullOrEmpty(directory))
                {
                    Directory.CreateDirectory(directory);
                }

                File.WriteAllText(StorageLocationCache, JsonConvert.SerializeObject(index, Formatting.Indented));
            }
            catch (Exception e)
            {
                // Caching is a convenience. Failing to write it must not discard a good fetch.
                Console.WriteLine("Could not cache the offset index. " + e.Message);
            }
        }

        public GameOffsets FetchForHash(string sha256Hash)
        {
            indexTask.Wait();
            if (LocalOffsetIndex.ContainsKey(sha256Hash))
            {
                Console.WriteLine($"Loaded offsets: {LocalOffsetIndex[sha256Hash].Description}");
                return LocalOffsetIndex[sha256Hash];
            }
            else
            {
                var offsets = OffsetIndex.ContainsKey(sha256Hash) ? OffsetIndex[sha256Hash] : null;
                if (offsets is not null)
                {
                    Console.WriteLine($"Loaded offsets: {OffsetIndex[sha256Hash].Description}");
                }
                return offsets;
            }

        }

        public void refreshLocal()
        {
            if (File.Exists(StorageLocation))
            {
                LocalOffsetIndex = JsonConvert.DeserializeObject<Dictionary<string, GameOffsets>>(File.ReadAllText(StorageLocation));
            }
        }
        public void AddToLocalIndex(string gameHash, GameOffsets offset)
        {
            using StreamWriter sw = File.CreateText(StorageLocation);
            LocalOffsetIndex[gameHash] = offset;
            var serialized = JsonConvert.SerializeObject(LocalOffsetIndex, Formatting.Indented);
            sw.Write(serialized);
        }
    }

    public class GameOffsets
    {
        [JsonProperty(DefaultValueHandling = DefaultValueHandling.Include)]
        public string Description = "";

        public int AmongUsClientOffset { get; set; }

        public int GameDataOffset { get; set; }

        public int MeetingHudOffset { get; set; }

        public int GameStartManagerOffset { get; set; }

        public int HudManagerOffset { get; set; }

        public int ServerManagerOffset { get; set; }

        public int TempDataOffset { get; set; }

        public int GameOptionsOffset { get; set; }

        public int[] MeetingHudPtr { get; set; }
        public int[] MeetingHudCachePtrOffsets { get; set; }
        public int[] MeetingHudStateOffsets { get; set; }
        public int[] GameStateOffsets { get; set; }
        public int[] AllPlayerPtrOffsets { get; set; }
        public int[] AllPlayersOffsets { get; set; }
        public int[] PlayerCountOffsets { get; set; }
        public int[] ExiledPlayerIdOffsets { get; set; }
        public int[] RawGameOverReasonOffsets { get; set; }
        public int[] WinningPlayersPtrOffsets { get; set; }
        public int[] WinningPlayersOffsets { get; set; }
        public int[] WinningPlayerCountOffsets { get; set; }
        public int[] GameCodeOffsets { get; set; }
        public int[] PlayRegionOffsets { get; set; }
        public int[] PlayMapOffsets { get; set; }
        public int[] StringOffsets { get; set; }
        public bool isEpic { get; set; }
        public int AddPlayerPtr { get; set; }
        public int PlayerListPtr { get; set; }

        public PlayerInfoStructOffsets PlayerInfoStructOffsets { get; set; }
        public WinningPlayerDataStructOffsets WinningPlayerDataStructOffsets { get; set; }
        public PlayerOutfitStructOffsets PlayerOutfitStructOffsets { get; set; }
    }

    public class PlayerInfoStructOffsets
    {
        public int PlayerIDOffset { get; set; }
        public int[] OutfitsOffset { get; set; }
        public int PlayerLevelOffset { get; set; }
        public int DisconnectedOffset { get; set; }
        public int[] RoleTypeOffset { get; set; }
        public int[] RoleTeamTypeOffset { get; set; }
        public int TasksOffset { get; set; }
        public int IsDeadOffset { get; set; }
        public int ObjectOffset { get; set; }
    }

    public class WinningPlayerDataStructOffsets
    {
        public int PlayerNameOffset { get; set; }
        public int OutfitOffset { get; set; }
        public int IsYouOffset { get; set; }
        public int IsImposterOffset { get; set; }
        public int IsDeadOffset { get; set; }
    }

    public class PlayerOutfitStructOffsets
    {
        public int ColorIDOffset { get; set; }
        public int HatIDOffset { get; set; }
        public int PetIDOffset { get; set; }
        public int SkinIDOffset { get; set; }
        public int VisorIDOffset { get; set; }
        public int NamePlateIDOffset { get; set; }
        public int PlayerNameOffset { get; set; }
    }
}
