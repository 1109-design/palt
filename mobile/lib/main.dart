import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'providers/trust_provider.dart';
import 'services/background.dart';
import 'theme/app_theme.dart';
import 'ui/screens/home_screen.dart';

/// Runs once per process, in the engine owned by PaltApplication — also at
/// boot with no window. The window attaches to the same engine when opened.
void main() async {
  WidgetsFlutterBinding.ensureInitialized();

  final sharedPrefs = await SharedPreferences.getInstance();
  final container = ProviderContainer(overrides: [
    sharedPreferencesProvider.overrideWithValue(sharedPrefs),
  ]);
  startCore(container);

  runApp(UncontrolledProviderScope(
    container: container,
    child: const PaltApp(),
  ));
}

class PaltApp extends StatelessWidget {
  const PaltApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'PALT',
      debugShowCheckedModeBanner: false,
      theme: buildAppTheme(),
      home: const HomeScreen(),
    );
  }
}
