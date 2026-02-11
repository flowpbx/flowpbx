import 'package:flutter/material.dart';
import 'package:flutter_animate/flutter_animate.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:flowpbx_mobile/models/directory_entry.dart';
import 'package:flowpbx_mobile/providers/directory_provider.dart';
import 'package:flowpbx_mobile/theme/color_tokens.dart';
import 'package:flowpbx_mobile/theme/dimensions.dart';
import 'package:flowpbx_mobile/theme/typography.dart';
import 'package:flowpbx_mobile/widgets/error_banner.dart';
import 'package:flowpbx_mobile/widgets/gradient_avatar.dart';
import 'package:flowpbx_mobile/widgets/skeleton_loader.dart';

class ContactsScreen extends ConsumerWidget {
  const ContactsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final directoryAsync = ref.watch(directoryProvider);

    return Scaffold(
      appBar: AppBar(
        title: const Text('Contacts'),
      ),
      body: directoryAsync.when(
        loading: () => const ContactsSkeleton(),
        error: (error, _) => ErrorBanner(
          error: error,
          fallbackMessage: 'Failed to load directory',
          onRetry: () => ref.invalidate(directoryProvider),
        ),
        data: (entries) => _DirectoryList(entries: entries),
      ),
    );
  }
}

class _DirectoryList extends StatefulWidget {
  final List<DirectoryEntry> entries;

  const _DirectoryList({required this.entries});

  @override
  State<_DirectoryList> createState() => _DirectoryListState();
}

class _DirectoryListState extends State<_DirectoryList> {
  final _searchController = TextEditingController();
  List<DirectoryEntry> _filtered = [];

  @override
  void initState() {
    super.initState();
    _filtered = widget.entries;
    _searchController.addListener(_onSearchChanged);
  }

  @override
  void didUpdateWidget(_DirectoryList oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.entries != widget.entries) {
      _onSearchChanged();
    }
  }

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  void _onSearchChanged() {
    final query = _searchController.text.toLowerCase().trim();
    setState(() {
      if (query.isEmpty) {
        _filtered = widget.entries;
      } else {
        _filtered = widget.entries.where((e) {
          return e.name.toLowerCase().contains(query) ||
              e.extension_.contains(query);
        }).toList();
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;

    // Sort and group alphabetically.
    final sorted = List<DirectoryEntry>.from(_filtered)
      ..sort((a, b) => a.name.toLowerCase().compareTo(b.name.toLowerCase()));

    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.all(Dimensions.space12),
          child: TextField(
            controller: _searchController,
            decoration: InputDecoration(
              hintText: 'Search by name or extension',
              prefixIcon: const Icon(Icons.search),
              suffixIcon: _searchController.text.isNotEmpty
                  ? IconButton(
                      icon: const Icon(Icons.clear),
                      onPressed: () => _searchController.clear(),
                    )
                  : null,
              contentPadding:
                  const EdgeInsets.symmetric(horizontal: Dimensions.space16),
            ),
          ),
        ),
        Expanded(
          child: sorted.isEmpty
              ? Center(
                  child: Text(
                    'No contacts found',
                    style: Theme.of(context).textTheme.bodyLarge?.copyWith(
                          color: colorScheme.onSurfaceVariant,
                        ),
                  ),
                )
              : _buildGroupedList(context, sorted),
        ),
      ],
    );
  }

  Widget _buildGroupedList(
      BuildContext context, List<DirectoryEntry> sorted) {
    // Build items with sticky section headers.
    final items = <_ListItem>[];
    String? lastLetter;
    for (final entry in sorted) {
      final letter = entry.name.isNotEmpty
          ? entry.name[0].toUpperCase()
          : '#';
      if (letter != lastLetter) {
        items.add(_ListItem.header(letter));
        lastLetter = letter;
      }
      items.add(_ListItem.entry(entry));
    }

    return ListView.builder(
      itemCount: items.length,
      itemBuilder: (context, index) {
        final item = items[index];
        if (item.isHeader) {
          return Container(
            width: double.infinity,
            padding: const EdgeInsets.fromLTRB(
              Dimensions.space16,
              Dimensions.space12,
              Dimensions.space16,
              Dimensions.space4,
            ),
            color: Theme.of(context).scaffoldBackgroundColor,
            child: Text(
              item.headerTitle!,
              style: Theme.of(context).textTheme.labelLarge?.copyWith(
                    color: Theme.of(context).colorScheme.primary,
                    fontWeight: FontWeight.w700,
                  ),
            ),
          );
        }
        return _ContactTile(entry: item.entry!);
      },
    );
  }
}

class _ListItem {
  final String? headerTitle;
  final DirectoryEntry? entry;

  _ListItem._({this.headerTitle, this.entry});

  factory _ListItem.header(String title) =>
      _ListItem._(headerTitle: title);
  factory _ListItem.entry(DirectoryEntry entry) =>
      _ListItem._(entry: entry);

  bool get isHeader => headerTitle != null;
}

class _ContactTile extends StatelessWidget {
  final DirectoryEntry entry;

  const _ContactTile({required this.entry});

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;

    return ListTile(
      leading: Stack(
        clipBehavior: Clip.none,
        children: [
          GradientAvatar(name: entry.name, radius: Dimensions.avatarRadiusMedium),
          Positioned(
            right: -2,
            bottom: -2,
            child: _PresenceDot(online: entry.online),
          ),
        ],
      ),
      title: Text(entry.name),
      subtitle: Text(
        entry.online
            ? 'Ext. ${entry.extension_} — Online'
            : 'Ext. ${entry.extension_} — Offline',
        style: TextStyle(
          fontSize: 13,
          color: colorScheme.onSurfaceVariant,
        ),
      ),
      trailing: IconButton(
        icon: Icon(Icons.call, color: ColorTokens.callGreen),
        tooltip: 'Call ${entry.extension_}',
        onPressed: () {
          context.go('/dialpad?number=${entry.extension_}');
        },
      ),
    );
  }
}

/// Online presence dot with pulse animation when online.
class _PresenceDot extends StatelessWidget {
  final bool online;

  const _PresenceDot({required this.online});

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;

    Widget dot = Container(
      width: 14,
      height: 14,
      decoration: BoxDecoration(
        color: online ? ColorTokens.registeredGreen : ColorTokens.offlineGrey,
        shape: BoxShape.circle,
        border: Border.all(
          color: colorScheme.surface,
          width: 2,
        ),
      ),
    );

    if (online) {
      dot = dot
          .animate(onPlay: (c) => c.repeat(reverse: true))
          .scaleXY(begin: 1.0, end: 0.8, duration: 1500.ms);
    }

    return dot;
  }
}
