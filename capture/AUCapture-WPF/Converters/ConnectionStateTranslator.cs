using System;
using System.Collections.Generic;
using System.Globalization;
using System.Linq;
using System.Text;
using System.Threading.Tasks;
using System.Windows.Data;
using AmongUsCapture;
using AUCapture_WPF.Properties;
using Gu.Localization;

namespace AUCapture_WPF.Converters
{
    class ConnectionStateTranslator : IMultiValueConverter
    {
        // The connection's name, the current culture so a change of language rewrites
        // it, and whether it is connected. The picture beside it tells that only by
        // its colour, so the words say it too (#135).
        public object Convert(object[] values, Type targetType, object parameter, CultureInfo culture)
        {
            if (values.Length < 2 || values[0] is not string name)
            {
                return string.Empty;
            }
            try
            {
                var label = TranslationFor("ConnectionStatus." + name.ToUpper()).Translated;
                if (values.Length < 3 || values[2] is not bool connected)
                {
                    return label;
                }
                var state = TranslationFor(connected ? "ConnectionStatus.Connected" : "ConnectionStatus.Disconnected").Translated;
                return label + ": " + state;
            }
            catch (Exception)
            {
                return string.Empty;
            }
        }
        public static ITranslation TranslationFor(string key, ErrorHandling errorHandling = ErrorHandling.ReturnErrorInfoPreserveNeutral)
        {
            return Gu.Localization.Translation.GetOrCreate(Resources.ResourceManager, key, errorHandling);
        }
        public object[] ConvertBack(object value, Type[] targetTypes, object parameter, CultureInfo culture)
        {
            throw new NotImplementedException();
        }
    }
}
