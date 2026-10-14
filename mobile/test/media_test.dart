import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:palt_mobile/utils/media.dart';

void main() {
  test('uniqueFilePath never overwrites', () {
    final dir = Directory.systemTemp.createTempSync('palt_media');
    addTearDown(() => dir.deleteSync(recursive: true));
    expect(uniqueFilePath(dir.path, 'a.jpg'), '${dir.path}/a.jpg');
    File('${dir.path}/a.jpg').writeAsStringSync('x');
    expect(uniqueFilePath(dir.path, 'a.jpg'), '${dir.path}/a (1).jpg');
    File('${dir.path}/a (1).jpg').writeAsStringSync('x');
    expect(uniqueFilePath(dir.path, 'a.jpg'), '${dir.path}/a (2).jpg');
    File('${dir.path}/README').writeAsStringSync('x');
    expect(uniqueFilePath(dir.path, 'README'), '${dir.path}/README (1)');
  });

  test('media kinds and durations', () {
    expect(isImageName('IMG_1.JPG'), isTrue);
    expect(isVideoName('clip.MP4'), isTrue);
    expect(isMediaName('notes.pdf'), isFalse);
    expect(formatDuration(const Duration(seconds: 83)), '1:23');
    expect(formatDuration(const Duration(hours: 1, seconds: 3)), '1:00:03');
  });
}
