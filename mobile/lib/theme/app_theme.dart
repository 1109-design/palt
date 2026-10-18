/// app_theme.dart — Beacon design tokens + ThemeData (see /design.md).
///
/// True-black background, near-black gradient surfaces, signal-amber accent,
/// mono data readouts.
///   - Bricolage Grotesque: display / headings / device names
///   - Inter: UI body
///   - JetBrains Mono: every wire-derived value (IPs, sizes, speeds, versions)
library;

import 'package:flutter/material.dart';
import 'package:google_fonts/google_fonts.dart';

// ── Beacon palette ───────────────────────────────────────────────────────────
const kBg            = Color(0xFF000000);
const kSurface       = Color(0xFF111216);
const kSurfaceRaised = Color(0xFF1B1D23);
const kHairline      = Color(0xFF26282F);
const kText          = Color(0xFFF2F3F6);
const kTextSecondary = Color(0xFF9A9EA9);
const kTextFaint     = Color(0xFF5C606B);
const kAccent        = Color(0xFFFFB454);
const kAccentPressed = Color(0xFFE69A38);
const kOnAccent      = Color(0xFF201503);
const kOk            = Color(0xFF53D993);
const kDanger        = Color(0xFFFF7B72);
/// Route tag for peers/transfers going through the relay server.
const kRelay         = Color(0xFF7AA2FF);
/// Action tiles and received bubbles.
const kTile          = Color(0xFF14151A);
/// Sent bubbles: dark amber, with light amber text and meta.
const kSent          = Color(0xFF3A2B12);
const kOnSent        = Color(0xFFFFE6C2);
const kSentMeta      = Color(0xFFC9A46A);

// ── Gradients ────────────────────────────────────────────────────────────────

/// Hero cards: a warm amber breath in the corner, fading to black.
const kCardGradient = LinearGradient(
  begin: Alignment.topLeft,
  end: Alignment.bottomRight,
  colors: [Color(0xFF2A2116), Color(0xFF15161A), Color(0xFF0B0B0D)],
  stops: [0, 0.45, 1],
);

/// Tiles and secondary surfaces.
const kTileGradient = LinearGradient(
  begin: Alignment.topLeft,
  end: Alignment.bottomRight,
  colors: [Color(0xFF1C1E24), Color(0xFF111216)],
);

/// Primary amber actions.
const kAccentGradient = LinearGradient(
  begin: Alignment.topLeft,
  end: Alignment.bottomRight,
  colors: [Color(0xFFFFCB7A), Color(0xFFFFA63D)],
);

/// A soft amber glow at the top of a page, over the black.
const kPageGlow = RadialGradient(
  center: Alignment(0, -1.15),
  radius: 1.1,
  colors: [Color(0x26FFB454), Color(0x00FFB454)],
);

// ── Type helpers ─────────────────────────────────────────────────────────────

/// Display voice — device names, screen titles, the wordmark.
TextStyle display({
  double fontSize = 24,
  FontWeight fontWeight = FontWeight.w700,
  Color color = kText,
  double letterSpacing = -0.5,
}) =>
    GoogleFonts.bricolageGrotesque(
      fontSize: fontSize,
      fontWeight: fontWeight,
      color: color,
      letterSpacing: letterSpacing,
    );

/// Data voice — anything that came off the wire.
TextStyle mono({
  double fontSize = 12,
  FontWeight fontWeight = FontWeight.w500,
  Color color = kTextSecondary,
}) =>
    GoogleFonts.jetBrainsMono(
      fontSize: fontSize,
      fontWeight: fontWeight,
      color: color,
    );

/// Overline voice — tiny uppercase section labels.
TextStyle overline({Color color = kTextFaint}) => GoogleFonts.inter(
      fontSize: 11,
      fontWeight: FontWeight.w600,
      letterSpacing: 1.6,
      color: color,
    );

ThemeData buildAppTheme() {
  final base = ThemeData(brightness: Brightness.dark, useMaterial3: true);

  return base.copyWith(
    colorScheme: const ColorScheme.dark(
      primary: kAccent,
      onPrimary: kOnAccent,
      secondary: kTextSecondary,
      onSecondary: kBg,
      tertiary: kOk,
      onTertiary: kBg,
      surface: kSurface,
      onSurface: kText,
      surfaceContainerLowest: kBg,
      surfaceContainerLow: kSurface,
      surfaceContainer: kSurface,
      surfaceContainerHigh: kSurfaceRaised,
      surfaceContainerHighest: kSurfaceRaised,
      outline: kHairline,
      outlineVariant: kHairline,
      error: kDanger,
      onError: kBg,
    ),
    textTheme: GoogleFonts.interTextTheme(base.textTheme).apply(
      bodyColor: kText,
      displayColor: kText,
    ),
    scaffoldBackgroundColor: kBg,
    appBarTheme: AppBarTheme(
      backgroundColor: kBg,
      foregroundColor: kText,
      elevation: 0,
      scrolledUnderElevation: 0,
      centerTitle: false,
      titleTextStyle: display(fontSize: 22, fontWeight: FontWeight.w800),
    ),
    cardTheme: const CardThemeData(
      elevation: 0,
      color: kSurface,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.all(Radius.circular(14)),
        side: BorderSide(color: kHairline),
      ),
      margin: EdgeInsets.zero,
    ),
    filledButtonTheme: FilledButtonThemeData(
      style: FilledButton.styleFrom(
        backgroundColor: kAccent,
        foregroundColor: kOnAccent,
        padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 13),
        shape: const RoundedRectangleBorder(
          borderRadius: BorderRadius.all(Radius.circular(10)),
        ),
        textStyle: GoogleFonts.inter(fontWeight: FontWeight.w600, fontSize: 15),
      ),
    ),
    outlinedButtonTheme: OutlinedButtonThemeData(
      style: OutlinedButton.styleFrom(
        foregroundColor: kText,
        side: const BorderSide(color: kHairline),
        padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 13),
        shape: const RoundedRectangleBorder(
          borderRadius: BorderRadius.all(Radius.circular(10)),
        ),
        textStyle: GoogleFonts.inter(fontWeight: FontWeight.w600, fontSize: 15),
      ),
    ),
    textButtonTheme: TextButtonThemeData(
      style: TextButton.styleFrom(
        foregroundColor: kTextSecondary,
        textStyle: GoogleFonts.inter(fontWeight: FontWeight.w600),
      ),
    ),
    floatingActionButtonTheme: FloatingActionButtonThemeData(
      backgroundColor: kAccent,
      foregroundColor: kOnAccent,
      elevation: 0,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.all(Radius.circular(14)),
      ),
      extendedTextStyle: GoogleFonts.inter(fontWeight: FontWeight.w600, fontSize: 15),
    ),
    dialogTheme: const DialogThemeData(
      backgroundColor: kSurfaceRaised,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.all(Radius.circular(18)),
        side: BorderSide(color: kHairline),
      ),
    ),
    bottomSheetTheme: const BottomSheetThemeData(
      backgroundColor: kSurfaceRaised,
      modalBackgroundColor: kSurfaceRaised,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(22)),
      ),
    ),
    dividerTheme: const DividerThemeData(color: kHairline, thickness: 1),
    checkboxTheme: CheckboxThemeData(
      side: const BorderSide(color: kTextFaint, width: 1.5),
      fillColor: WidgetStateProperty.resolveWith(
        (states) => states.contains(WidgetState.selected) ? kAccent : null,
      ),
      checkColor: const WidgetStatePropertyAll(kOnAccent),
    ),
    progressIndicatorTheme: const ProgressIndicatorThemeData(
      color: kAccent,
      linearTrackColor: kHairline,
      circularTrackColor: kHairline,
    ),
    snackBarTheme: SnackBarThemeData(
      backgroundColor: kSurfaceRaised,
      contentTextStyle: GoogleFonts.inter(color: kText, fontSize: 13),
      behavior: SnackBarBehavior.floating,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.all(Radius.circular(10)),
        side: BorderSide(color: kHairline),
      ),
    ),
    inputDecorationTheme: InputDecorationTheme(
      filled: true,
      fillColor: kBg,
      hintStyle: GoogleFonts.inter(color: kTextFaint),
      helperStyle: GoogleFonts.inter(color: kTextFaint, fontSize: 11),
      enabledBorder: const OutlineInputBorder(
        borderRadius: BorderRadius.all(Radius.circular(10)),
        borderSide: BorderSide(color: kHairline),
      ),
      focusedBorder: const OutlineInputBorder(
        borderRadius: BorderRadius.all(Radius.circular(10)),
        borderSide: BorderSide(color: kAccent),
      ),
    ),
  );
}
