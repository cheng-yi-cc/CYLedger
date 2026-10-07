package com.cyledger.android;

import android.content.Context;
import android.net.ConnectivityManager;
import android.net.LinkProperties;
import android.net.Network;
import android.net.NetworkCapabilities;
import android.net.NetworkRequest;
import java.net.InetAddress;
import java.util.HashSet;
import java.util.Set;

/** Process-lifetime observer. Only public-market sockets use the selected network. */
final class MarketNetwork {
    private static MarketNetwork instance;
    private final ConnectivityManager manager;
    private final Set<Network> blocked = new HashSet<>();

    static synchronized void start(Context context) {
        if (instance != null) return;
        MarketNetwork observer = new MarketNetwork(context.getApplicationContext());
        instance = observer;
        observer.register();
    }

    private MarketNetwork(Context context) {
        manager = (ConnectivityManager) context.getSystemService(Context.CONNECTIVITY_SERVICE);
    }

    private void register() {
        if (manager == null) return;
        try {
            manager.registerNetworkCallback(new NetworkRequest.Builder()
                .addCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET)
                .addCapability(NetworkCapabilities.NET_CAPABILITY_NOT_VPN).build(), physical);
            // Also invalidate pooled connections on a VPN transition.
            manager.registerDefaultNetworkCallback(defaultNetwork);
            refresh();
        } catch (SecurityException ignored) {
            NativeBridge.directNetwork(0L, "");
        }
    }

    private final ConnectivityManager.NetworkCallback physical = new ConnectivityManager.NetworkCallback() {
        @Override public void onAvailable(Network network) { refresh(); }
        @Override public void onCapabilitiesChanged(Network network, NetworkCapabilities caps) { refresh(); }
        @Override public void onLinkPropertiesChanged(Network network, LinkProperties properties) { refresh(); }
        @Override public void onLost(Network network) { synchronized (MarketNetwork.this) { blocked.remove(network); } refresh(); }
        @Override public void onBlockedStatusChanged(Network network, boolean isBlocked) {
            synchronized (MarketNetwork.this) { if (isBlocked) blocked.add(network); else blocked.remove(network); }
            refresh();
        }
    };
    private final ConnectivityManager.NetworkCallback defaultNetwork = new ConnectivityManager.NetworkCallback() {
        @Override public void onAvailable(Network network) { refresh(); }
        @Override public void onLost(Network network) { refresh(); }
    };

    private synchronized void refresh() {
        try {
            Network selected = null;
            LinkProperties selectedProperties = null;
            int best = -1;
            for (Network network : manager.getAllNetworks()) {
                NetworkCapabilities caps = manager.getNetworkCapabilities(network);
                LinkProperties properties = manager.getLinkProperties(network);
                if (blocked.contains(network) || caps == null || properties == null || properties.getDnsServers().isEmpty()
                    || !caps.hasCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET)
                    || !caps.hasCapability(NetworkCapabilities.NET_CAPABILITY_VALIDATED)
                    || !caps.hasCapability(NetworkCapabilities.NET_CAPABILITY_NOT_VPN)
                    || caps.hasTransport(NetworkCapabilities.TRANSPORT_VPN)) continue;
                int score = caps.hasTransport(NetworkCapabilities.TRANSPORT_ETHERNET) ? 3
                    : caps.hasTransport(NetworkCapabilities.TRANSPORT_WIFI) ? 2 : 1;
                if (score > best) { selected = network; selectedProperties = properties; best = score; }
            }
            StringBuilder dns = new StringBuilder();
            if (selectedProperties != null) for (InetAddress address : selectedProperties.getDnsServers()) {
                if (dns.length() > 0) dns.append(' ');
                dns.append(address.getHostAddress());
            }
            NativeBridge.directNetwork(selected == null ? 0L : selected.getNetworkHandle(), dns.toString());
        } catch (SecurityException ignored) {
            NativeBridge.directNetwork(0L, "");
        }
    }
}
