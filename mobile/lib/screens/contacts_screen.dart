import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:flowpbx_mobile/models/directory_entry.dart';
import 'package:flowpbx_mobile/providers/directory_provider.dart';
import 'package:flowpbx_mobile/widgets/error_banner.dart';
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

    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.all(12),
          child: TextField(
            controller: _searchController,
            decoration: InputDecoration(
              hintText: 'Search by name or extension',
              prefixIcon: const Icon(Icons.search),
              suffixIcon: _searchController.text.isNotEmpty
                  ? IconButton(
                      icon: const Icon(Icons.clear),
                      onPressed: () {
                        _searchController.clear();
                      },
                    )
                  : null,
              border: OutlineInputBorder(
                borderRadius: BorderRadius.circular(12),
              ),
              filled: true,
              fillColor: colorScheme.surfaceContainerHighest.withOpacity(0.3),
              contentPadding: const EdgeInsets.symmetric(horizontal: 16),
            ),
          ),
        ),
        Expanded(
          child: _filtered.isEmpty
              ? Center(
                  child: Text(
                    'No contacts found',
                    style: Theme.of(context).textTheme.bodyLarge?.copyWith(
                          color: colorScheme.onSurfaceVariant,
                        ),
                  ),
                )
              : ListView.separated(
                  itemCount: _filtered.length,
                  separatorBuilder: (_, __) => const Divider(height: 1),
                  itemBuilder: (context, index) {
                    final entry = _filtered[index];
                    return _ContactTile(entry: entry);
                  },
                ),
        ),
      ],
    );
  }
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
          CircleAvatar(
            backgroundColor: colorScheme.primaryContainer,
            child: Text(
              _initials(entry.name),
              style: TextStyle(
                color: colorScheme.onPrimaryContainer,
                fontWeight: FontWeight.w600,
              ),
            ),
          ),
          Positioned(
            right: -2,
            bottom: -2,
            child: Container(
              width: 14,
              height: 14,
              decoration: BoxDecoration(
                color: entry.online ? Colors.green : Colors.grey,
                shape: BoxShape.circle,
                border: Border.all(
                  color: colorScheme.surface,
                  width: 2,
                ),
              ),
            ),
          ),
        ],
      ),
      title: Text(entry.name),
      subtitle: Text(
        entry.online
            ? 'Ext. ${entry.extension_} — Online'
            : 'Ext. ${entry.extension_} — Offline',
      ),
      trailing: IconButton(
        icon: const Icon(Icons.call, color: Colors.green),
        tooltip: 'Call ${entry.extension_}',
        onPressed: () {
          context.go('/dialpad?number=${entry.extension_}');
        },
      ),
    );
  }

  String _initials(String name) {
    final parts = name.trim().split(RegExp(r'\s+'));
    if (parts.length >= 2) {
      return '${parts.first[0]}${parts.last[0]}'.toUpperCase();
    }
    return name.isNotEmpty ? name[0].toUpperCase() : '?';
  }
}
