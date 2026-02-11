import 'package:path/path.dart';
import 'package:path_provider/path_provider.dart';
import 'package:sqflite/sqflite.dart';

import 'package:flowpbx_mobile/models/call_history_entry.dart';

/// Local SQLite cache for call history entries.
///
/// Provides offline access to previously fetched call records and faster
/// initial load times by returning cached data before the API responds.
class CallHistoryCacheService {
  Database? _db;

  /// Open (or create) the local cache database.
  Future<Database> _getDb() async {
    if (_db != null) return _db!;
    final dir = await getApplicationDocumentsDirectory();
    final path = join(dir.path, 'call_history_cache.db');
    _db = await openDatabase(
      path,
      version: 1,
      onCreate: (db, version) async {
        await db.execute('''
          CREATE TABLE call_history (
            id INTEGER PRIMARY KEY,
            call_id TEXT NOT NULL,
            start_time TEXT NOT NULL,
            answer_time TEXT,
            end_time TEXT,
            duration INTEGER,
            caller_id_name TEXT NOT NULL DEFAULT '',
            caller_id_num TEXT NOT NULL DEFAULT '',
            callee TEXT NOT NULL DEFAULT '',
            direction TEXT NOT NULL DEFAULT '',
            disposition TEXT NOT NULL DEFAULT ''
          )
        ''');
        await db.execute(
          'CREATE INDEX idx_ch_start_time ON call_history(start_time)',
        );
      },
    );
    return _db!;
  }

  /// Replace all cached entries with the given list (full refresh strategy).
  Future<void> replaceAll(List<CallHistoryEntry> entries) async {
    final db = await _getDb();
    await db.transaction((txn) async {
      await txn.delete('call_history');
      for (final entry in entries) {
        await txn.insert('call_history', _toRow(entry));
      }
    });
  }

  /// Upsert entries into the cache (insert or replace by id).
  /// Used when appending paginated results without wiping existing data.
  Future<void> upsertAll(List<CallHistoryEntry> entries) async {
    final db = await _getDb();
    await db.transaction((txn) async {
      for (final entry in entries) {
        await txn.insert(
          'call_history',
          _toRow(entry),
          conflictAlgorithm: ConflictAlgorithm.replace,
        );
      }
    });
  }

  /// Return all cached entries, newest first.
  Future<List<CallHistoryEntry>> getAll() async {
    final db = await _getDb();
    final rows = await db.query(
      'call_history',
      orderBy: 'start_time DESC',
    );
    return rows.map(_fromRow).toList();
  }

  /// Close the database (e.g. on logout).
  Future<void> close() async {
    await _db?.close();
    _db = null;
  }

  Map<String, Object?> _toRow(CallHistoryEntry e) => {
        'id': e.id,
        'call_id': e.callId,
        'start_time': e.startTime.toIso8601String(),
        'answer_time': e.answerTime?.toIso8601String(),
        'end_time': e.endTime?.toIso8601String(),
        'duration': e.duration,
        'caller_id_name': e.callerIdName,
        'caller_id_num': e.callerIdNum,
        'callee': e.callee,
        'direction': e.direction,
        'disposition': e.disposition,
      };

  CallHistoryEntry _fromRow(Map<String, Object?> row) => CallHistoryEntry(
        id: row['id'] as int,
        callId: row['call_id'] as String,
        startTime: DateTime.parse(row['start_time'] as String),
        answerTime: row['answer_time'] != null
            ? DateTime.parse(row['answer_time'] as String)
            : null,
        endTime: row['end_time'] != null
            ? DateTime.parse(row['end_time'] as String)
            : null,
        duration: row['duration'] as int?,
        callerIdName: row['caller_id_name'] as String? ?? '',
        callerIdNum: row['caller_id_num'] as String? ?? '',
        callee: row['callee'] as String? ?? '',
        direction: row['direction'] as String? ?? '',
        disposition: row['disposition'] as String? ?? '',
      );
}
